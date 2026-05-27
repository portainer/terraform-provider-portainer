package internal

import (
	"net/http"
	"strings"
	"testing"
)

// TestCheckCreate_Standalone exercises the standalone (non-Swarm) path.
// /docker/swarm returns 404 → resource falls back to listing containers
// and verifying revision/state.
func TestCheckCreate_Standalone(t *testing.T) {
	mock := NewMockServer(t)

	// Swarm probe — non-200 → standalone branch.
	mock.On("GET", "/endpoints/1/docker/swarm", RespondString(
		http.StatusNotFound, "application/json",
		`{"message":"not a swarm"}`,
	))

	// Container list with a single container whose name contains the
	// expanded service name and whose image tag matches the revision.
	mock.On("GET", "/endpoints/1/docker/containers/json", RespondJSON(http.StatusOK, []map[string]interface{}{
		{
			"Names": []interface{}{"/mystack_web.1"},
			"State": "running",
			"Image": "nginx:1.25",
		},
	}))

	r := resourceCheck()
	d := r.TestResourceData()
	_ = d.Set("endpoint_id", 1)
	_ = d.Set("stack_name", "mystack")
	_ = d.Set("revision", "1.25")
	_ = d.Set("services_list", "web")
	_ = d.Set("desired_state", "running")
	_ = d.Set("wait", 0)
	_ = d.Set("wait_between_checks", 0)
	_ = d.Set("max_retries", 1)

	if err := r.Create(d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	if d.Id() == "" {
		t.Error("expected non-empty ID after Create")
	}
	if !strings.HasPrefix(d.Id(), "check-") {
		t.Errorf("ID: expected prefix \"check-\", got %q", d.Id())
	}
	if got := d.Get("output").(string); !strings.Contains(got, "Docker Standalone detected") {
		t.Errorf("output: expected to mention Docker Standalone, got %q", got)
	}
}

// TestCheckCreate_StandaloneRetryFails verifies the resource returns an
// error if no container matches after the configured retries.
func TestCheckCreate_StandaloneRetryFails(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/endpoints/1/docker/swarm", RespondString(
		http.StatusNotFound, "application/json",
		`{"message":"not a swarm"}`,
	))

	// Container exists but tag/state don't match.
	mock.On("GET", "/endpoints/1/docker/containers/json", RespondJSON(http.StatusOK, []map[string]interface{}{
		{
			"Names": []interface{}{"/mystack_web.1"},
			"State": "exited",
			"Image": "nginx:1.20",
		},
	}))

	r := resourceCheck()
	d := r.TestResourceData()
	_ = d.Set("endpoint_id", 1)
	_ = d.Set("stack_name", "mystack")
	_ = d.Set("revision", "1.25")
	_ = d.Set("services_list", "web")
	_ = d.Set("desired_state", "running")
	_ = d.Set("wait", 0)
	_ = d.Set("wait_between_checks", 0)
	_ = d.Set("max_retries", 1)

	err := r.Create(d, mock.Client())
	if err == nil {
		t.Fatal("expected error when no container matches revision/state")
	}
}

// TestCheckCreate_Swarm exercises the swarm path. The /docker/swarm probe
// returns the cluster info, then the tasks list contains one task matching
// the revision and desired-state.
func TestCheckCreate_Swarm(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/endpoints/1/docker/swarm", RespondJSON(http.StatusOK, map[string]interface{}{
		"ID": "swarm-cluster-id",
	}))
	mock.On("GET", "/endpoints/1/docker/tasks", RespondJSON(http.StatusOK, []map[string]interface{}{
		{
			"Spec": map[string]interface{}{
				"ContainerSpec": map[string]interface{}{"Image": "nginx:1.25"},
			},
			"Status": map[string]interface{}{"State": "running"},
		},
	}))

	r := resourceCheck()
	d := r.TestResourceData()
	_ = d.Set("endpoint_id", 1)
	_ = d.Set("stack_name", "mystack")
	_ = d.Set("revision", "1.25")
	_ = d.Set("services_list", "web")
	_ = d.Set("desired_state", "running")
	_ = d.Set("wait", 0)
	_ = d.Set("wait_between_checks", 0)
	_ = d.Set("max_retries", 1)

	if err := r.Create(d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if got := d.Get("output").(string); !strings.Contains(got, "Docker Swarm detected") {
		t.Errorf("output: expected to mention Docker Swarm, got %q", got)
	}
}

// TestCheckRead_NoOp verifies Read is stateless (no HTTP calls).
func TestCheckRead_NoOp(t *testing.T) {
	mock := NewMockServer(t)

	r := resourceCheck()
	d := r.TestResourceData()
	d.SetId("check-123")

	if err := r.Read(d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if len(mock.Requests()) != 0 {
		t.Errorf("expected zero HTTP requests during Read, got %d", len(mock.Requests()))
	}
}

// TestCheckDelete_ClearsID verifies Delete is purely local (no HTTP) and
// clears the ID.
func TestCheckDelete_ClearsID(t *testing.T) {
	mock := NewMockServer(t)

	r := resourceCheck()
	d := r.TestResourceData()
	d.SetId("check-123")

	if err := r.Delete(d, mock.Client()); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	if d.Id() != "" {
		t.Errorf("expected ID cleared, got %q", d.Id())
	}
	if len(mock.Requests()) != 0 {
		t.Errorf("expected zero HTTP requests during Delete, got %d", len(mock.Requests()))
	}
}
