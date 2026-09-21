package internal

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

// =========================================================================
// Coverage for issue #145: Portainer 2.45 deploys stacks asynchronously and
// holds them in status 3 (Deploying) until the deploy finishes, rejecting every
// mutating call in that window with 409 "Stack deployment is already in
// progress". The provider's finalize PUT landed inside that window on every
// create, so creating a standalone or swarm stack failed 100% of the time even
// though the stack itself came up fine.
// =========================================================================

// withFastStackPolling shrinks the settle poll interval so these tests do not
// wait in real time.
func withFastStackPolling(t *testing.T) {
	t.Helper()
	original := stackSettlePollInterval
	stackSettlePollInterval = time.Millisecond
	t.Cleanup(func() { stackSettlePollInterval = original })
}

// respondStackStatuses answers GET /stacks/{id} with the given statuses in
// order, repeating the last one once they run out. That is how a real deploy
// looks to a poller: Deploying for a while, then something terminal.
func respondStackStatuses(statuses ...int) http.HandlerFunc {
	var calls int64
	return func(w http.ResponseWriter, _ *http.Request) {
		i := int(atomic.AddInt64(&calls, 1)) - 1
		if i >= len(statuses) {
			i = len(statuses) - 1
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"Id": 5, "Name": "web", "Status": statuses[i], "Type": 2, "EndpointId": 1,
		})
	}
}

// stackCreateResourceData builds the configuration the issue reports on:
// a standalone stack deployed from a string, with prune enabled.
func stackCreateResourceData(t *testing.T) (*schema.Resource, *schema.ResourceData) {
	t.Helper()
	r := resourcePortainerStack()
	d := r.TestResourceData()
	_ = d.Set("deployment_type", "standalone")
	_ = d.Set("method", "string")
	_ = d.Set("name", "web")
	_ = d.Set("endpoint_id", 1)
	_ = d.Set("stack_file_content", "version: '3'")
	_ = d.Set("prune", true)
	// Without this the create chains into a stop call, which is a different
	// code path than the one under test here.
	_ = d.Set("active", true)
	return r, d
}

// TestStackCreate_WaitsOutAsyncDeployment is the regression test: the finalize
// PUT must not be sent while the stack is still deploying, and the create must
// succeed rather than tainting a stack that deployed fine.
func TestStackCreate_WaitsOutAsyncDeployment(t *testing.T) {
	withFastStackPolling(t)

	mock := NewMockServer(t)
	mockEmptyStackList(mock)

	mock.On("POST", "/stacks/create/standalone/string", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id": 5, "Name": "web",
	}))
	// Deploying twice, then Active — the shape of a real async deploy.
	mock.On("GET", "/stacks/5", respondStackStatuses(
		stackStatusDeploying, stackStatusDeploying, 1,
	))
	mock.On("PUT", "/stacks/5", RespondJSON(http.StatusOK, map[string]interface{}{"Id": 5, "Name": "web"}))
	mock.On("GET", "/stacks/5/file", RespondJSON(http.StatusOK, map[string]interface{}{
		"StackFileContent": "version: '3'",
	}))

	r, d := stackCreateResourceData(t)
	if err := rcCreate(r, d, mock.Client()); err != nil {
		t.Fatalf("Create must succeed once the deployment settles: %v", err)
	}

	// The stack was polled before the finalize PUT was attempted.
	var sawGet, sawPut bool
	for _, req := range mock.Requests() {
		if req.Method == http.MethodGet && req.Path == "/stacks/5" {
			sawGet = true
		}
		if req.Method == http.MethodPut && req.Path == "/stacks/5" {
			if !sawGet {
				t.Error("the finalize PUT was sent before the stack status was ever polled")
			}
			sawPut = true
		}
	}
	if !sawGet || !sawPut {
		t.Errorf("expected both a status poll and a finalize PUT, got poll=%v put=%v", sawGet, sawPut)
	}
}

// TestStackCreate_RetriesResidualConflict covers the window between the poll
// and the request: another client can start a deploy in it, so a conflict on
// the PUT itself is retried rather than failing the apply.
func TestStackCreate_RetriesResidualConflict(t *testing.T) {
	withFastStackPolling(t)

	mock := NewMockServer(t)
	mockEmptyStackList(mock)

	mock.On("POST", "/stacks/create/standalone/string", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id": 5, "Name": "web",
	}))
	mock.On("GET", "/stacks/5", respondStackStatuses(1))

	var puts int64
	mock.On("PUT", "/stacks/5", func(w http.ResponseWriter, _ *http.Request) {
		if atomic.AddInt64(&puts, 1) == 1 {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusConflict)
			_, _ = w.Write([]byte(`{"message":"Unable to update stack","details":"Stack deployment is already in progress"}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"Id":5,"Name":"web"}`))
	})
	mock.On("GET", "/stacks/5/file", RespondJSON(http.StatusOK, map[string]interface{}{
		"StackFileContent": "version: '3'",
	}))

	r, d := stackCreateResourceData(t)
	if err := rcCreate(r, d, mock.Client()); err != nil {
		t.Fatalf("a residual conflict must be retried, not fail the create: %v", err)
	}
	if got := atomic.LoadInt64(&puts); got != 2 {
		t.Errorf("expected the finalize PUT to be retried exactly once, got %d attempts", got)
	}
}

// TestStackCreate_FailedDeploymentIsReported is the case a blind retry of the
// PUT gets wrong. A deployment that FAILS also releases the lock, so retrying
// would eventually succeed and report a healthy apply for a broken stack.
// Waiting on the status catches it instead.
func TestStackCreate_FailedDeploymentIsReported(t *testing.T) {
	withFastStackPolling(t)

	mock := NewMockServer(t)
	mockEmptyStackList(mock)

	mock.On("POST", "/stacks/create/standalone/string", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id": 5, "Name": "web",
	}))
	mock.On("GET", "/stacks/5", respondStackStatuses(stackStatusDeploying, stackStatusError))
	// Would succeed if it were ever called — it must not be.
	mock.On("PUT", "/stacks/5", RespondJSON(http.StatusOK, map[string]interface{}{"Id": 5}))

	r, d := stackCreateResourceData(t)
	err := rcCreate(r, d, mock.Client())
	if err == nil {
		t.Fatal("a failed deployment must fail the create, not be reported as success")
	}
	if !strings.Contains(err.Error(), "deployment failed") {
		t.Errorf("the error should say the deployment failed, got: %v", err)
	}
	if mock.FindRequest("PUT", "/stacks/5") != nil {
		t.Error("no finalize PUT should be attempted after a failed deployment")
	}
}

// TestStackCreate_SynchronousPortainerIsUnaffected pins the backward
// compatibility: Portainer before 2.45 reports Active straight away (and never
// status 3), so the create must not poll in a loop or change behaviour.
func TestStackCreate_SynchronousPortainerIsUnaffected(t *testing.T) {
	withFastStackPolling(t)

	mock := NewMockServer(t)
	mockEmptyStackList(mock)

	mock.On("POST", "/stacks/create/standalone/string", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id": 5, "Name": "web",
	}))
	mock.On("GET", "/stacks/5", respondStackStatuses(1))
	mock.On("PUT", "/stacks/5", RespondJSON(http.StatusOK, map[string]interface{}{"Id": 5, "Name": "web"}))
	mock.On("GET", "/stacks/5/file", RespondJSON(http.StatusOK, map[string]interface{}{
		"StackFileContent": "version: '3'",
	}))

	r, d := stackCreateResourceData(t)
	if err := rcCreate(r, d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// One poll before the finalize PUT, plus the reads that Create chains into.
	// What matters is that it did not spin: a handful, not dozens.
	polls := 0
	for _, req := range mock.Requests() {
		if req.Method == http.MethodGet && req.Path == "/stacks/5" {
			polls++
		}
	}
	if polls == 0 || polls > 4 {
		t.Errorf("expected a small number of status reads on a synchronous Portainer, got %d", polls)
	}
}

// TestWaitForStackSettled_UnrelatedConflictNotRetried guards the retry
// predicate: 409 is also how Portainer answers other conflicts, and retrying
// those would spin until the timeout instead of reporting the real problem.
func TestWaitForStackSettled_UnrelatedConflictNotRetried(t *testing.T) {
	if isStackDeploymentConflict(http.StatusConflict, `{"message":"Unable to update stack","details":"Stack deployment is already in progress"}`) != true {
		t.Error("the deployment lock message must be recognised as retryable")
	}
	if isStackDeploymentConflict(http.StatusConflict, `{"message":"A stack with the same name already exists"}`) {
		t.Error("an unrelated 409 must not be treated as the deployment lock")
	}
	if isStackDeploymentConflict(http.StatusInternalServerError, "already in progress") {
		t.Error("only a 409 is the deployment lock")
	}
}

// TestWaitForStackSettled_GivesUpWithTimeout verifies a stack stuck in
// Deploying ends in a clear timeout that points at the resource timeout, rather
// than hanging or returning a bare context error.
func TestWaitForStackSettled_GivesUpWithTimeout(t *testing.T) {
	withFastStackPolling(t)

	mock := NewMockServer(t)
	mock.On("GET", "/stacks/9", respondStackStatuses(stackStatusDeploying))

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	err := waitForStackSettled(ctx, mock.Client(), "9")
	if err == nil {
		t.Fatal("expected a timeout for a stack stuck in Deploying")
	}
	if !strings.Contains(err.Error(), "gave up waiting") {
		t.Errorf("the error should name the timeout and the remedy, got: %v", err)
	}
}

// TestStackCreate_SwarmWaitsOutAsyncDeployment covers the half of issue #145
// the original report did not: silvercurls confirmed a swarm stack hits the
// same 409 and the same taint-then-recreate loop. The finalize PUT is shared
// by both deployment types, and this pins that it stays that way.
func TestStackCreate_SwarmWaitsOutAsyncDeployment(t *testing.T) {
	withFastStackPolling(t)

	mock := NewMockServer(t)
	mockEmptyStackList(mock)

	mock.On("POST", "/stacks/create/swarm/string", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id": 5, "Name": "web",
	}))
	// Deploying twice, then Active — the shape of a real async deploy.
	mock.On("GET", "/stacks/5", respondStackStatuses(
		stackStatusDeploying, stackStatusDeploying, 1,
	))
	mock.On("PUT", "/stacks/5", RespondJSON(http.StatusOK, map[string]interface{}{"Id": 5, "Name": "web"}))
	mock.On("GET", "/stacks/5/file", RespondJSON(http.StatusOK, map[string]interface{}{
		"StackFileContent": "version: '3'",
	}))

	r := resourcePortainerStack()
	d := r.TestResourceData()
	_ = d.Set("deployment_type", "swarm")
	_ = d.Set("method", "string")
	_ = d.Set("name", "web")
	_ = d.Set("endpoint_id", 1)
	_ = d.Set("swarm_id", "swarm-abc")
	_ = d.Set("stack_file_content", "version: '3'")
	_ = d.Set("prune", true)
	_ = d.Set("active", true)

	if err := rcCreate(r, d, mock.Client()); err != nil {
		t.Fatalf("a swarm create must survive the deployment lock too: %v", err)
	}

	var sawGet, sawPut bool
	for _, req := range mock.Requests() {
		if req.Method == http.MethodGet && req.Path == "/stacks/5" {
			sawGet = true
		}
		if req.Method == http.MethodPut && req.Path == "/stacks/5" {
			if !sawGet {
				t.Error("the finalize PUT was sent before the stack status was ever polled")
			}
			sawPut = true
		}
	}
	if !sawGet || !sawPut {
		t.Errorf("expected both a status poll and a finalize PUT, got poll=%v put=%v", sawGet, sawPut)
	}
}
