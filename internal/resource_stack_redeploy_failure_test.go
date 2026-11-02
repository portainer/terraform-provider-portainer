package internal

import (
	"encoding/json"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
)

// respondStackHistory answers GET /stacks/{id} with the given stack states in
// turn, repeating the last one, each with its Portainer 2.45 deployment history.
func respondStackHistory(states ...map[string]interface{}) http.HandlerFunc {
	var calls int64
	return func(w http.ResponseWriter, _ *http.Request) {
		i := int(atomic.AddInt64(&calls, 1)) - 1
		if i >= len(states) {
			i = len(states) - 1
		}
		body := map[string]interface{}{"Id": 3, "Name": "app", "Type": 2, "EndpointId": 1}
		for k, v := range states[i] {
			body[k] = v
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(body)
	}
}

func history(status int, entries ...map[string]interface{}) map[string]interface{} {
	return map[string]interface{}{"Status": status, "DeploymentStatus": entries}
}

func entry(status int, time int64, message string) map[string]interface{} {
	return map[string]interface{}{"Status": status, "Time": time, "Message": message}
}

func stackRedeployMock(t *testing.T, states ...map[string]interface{}) *MockServer {
	mock := NewMockServer(t)
	mock.On("GET", "/stacks/3", respondStackHistory(states...))
	mock.On("PUT", "/stacks/3", RespondJSON(http.StatusOK, map[string]interface{}{"Id": 3, "Name": "app"}))
	mock.On("GET", "/stacks/3/file", RespondJSON(http.StatusOK, map[string]interface{}{"StackFileContent": "version: '3'"}))
	return mock
}

// TestStackUpdate_ReportsFailedRedeploy: the update itself is accepted, then
// the asynchronous redeploy fails. The apply used to succeed and leave a stack
// that never came up.
func TestStackUpdate_ReportsFailedRedeploy(t *testing.T) {
	withFastStackPolling(t)
	mock := stackRedeployMock(t,
		history(1, entry(3, 100, ""), entry(1, 101, "")),
		history(3, entry(3, 200, "")),
		history(4, entry(3, 200, ""), entry(4, 203, "pull access denied for app:missing")),
	)

	r, d := stackUpdateResourceData(t, "3", true)
	err := rcUpdate(r, d, mock.Client())
	if err == nil || !strings.Contains(err.Error(), "pull access denied for app:missing") {
		t.Fatalf("expected the failed redeploy with Portainer's message, got %v", err)
	}
}

// TestStackUpdate_ErrorWithoutHistoryNotReported keeps issue #147 fixed on
// versions before 2.45: they keep no deployment history, so a stack left in
// Error by an earlier deployment cannot be told from a failed update and the
// update is not reported as failed.
func TestStackUpdate_ErrorWithoutHistoryNotReported(t *testing.T) {
	withFastStackPolling(t)
	mock := stackRedeployMock(t, history(4))

	r, d := stackUpdateResourceData(t, "3", true)
	if err := rcUpdate(r, d, mock.Client()); err != nil {
		t.Fatalf("an Error without deployment history must not fail the update: %v", err)
	}
}

// TestStackUpdate_ReportsFailureInSameSecond covers the history's one-second
// resolution: a redeploy that starts in the same second as the deployment
// before it, and fails, is still reported.
func TestStackUpdate_ReportsFailureInSameSecond(t *testing.T) {
	withFastStackPolling(t)
	mock := stackRedeployMock(t,
		history(1, entry(3, 100, ""), entry(1, 100, "")),
		history(4, entry(3, 100, ""), entry(4, 100, "no such image")),
	)

	r, d := stackUpdateResourceData(t, "3", true)
	err := rcUpdate(r, d, mock.Client())
	if err == nil || !strings.Contains(err.Error(), "no such image") {
		t.Fatalf("expected the failed redeploy to be reported, got %v", err)
	}
}

// TestStackUpdate_RecoversFailedStack is the #147 fix seen through the history:
// a stack in Error is redeployed successfully by the update.
func TestStackUpdate_RecoversFailedStack(t *testing.T) {
	withFastStackPolling(t)
	mock := stackRedeployMock(t,
		history(4, entry(3, 100, ""), entry(4, 101, "old failure")),
		history(1, entry(3, 200, ""), entry(1, 205, "")),
	)

	r, d := stackUpdateResourceData(t, "3", true)
	if err := rcUpdate(r, d, mock.Client()); err != nil {
		t.Fatalf("Update failed: %v", err)
	}
}
