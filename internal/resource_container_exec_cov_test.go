package internal

import (
	"net/http"
	"strings"
	"testing"
)

// =========================================================================
// Additional coverage for resource_container_exec.go. The exec happy paths
// require a real docker daemon to return a usable raw stream, so the existing
// _test.go already covers the standalone helper/error paths. Here we add the
// stateless Read contract and the swarm-mode error path (task lookup failure),
// which does not need a daemon.
// =========================================================================

// TestContainerExecRead_Stateless verifies Read is a no-op that preserves the
// ID (the resource is stateless server-side).
func TestContainerExecRead_Stateless(t *testing.T) {
	r := resourceContainerExec()
	d := r.TestResourceData()
	d.SetId("exec-keep")

	if err := rcRead(r, d, nil); err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if d.Id() != "exec-keep" {
		t.Errorf("Read should be a no-op; expected ID preserved, got %q", d.Id())
	}
}

// TestContainerExecCreate_Swarm_NoTasks verifies the swarm branch errors when
// the task list is empty (no daemon needed; the failure happens before exec).
func TestContainerExecCreate_Swarm_NoTasks(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/endpoints/1/docker/tasks", RespondJSON(http.StatusOK, []map[string]interface{}{}))

	r := resourceContainerExec()
	d := r.TestResourceData()
	_ = d.Set("endpoint_id", 1)
	_ = d.Set("service_name", "mysvc")
	_ = d.Set("command", "ls")
	_ = d.Set("mode", "swarm")

	err := rcCreate(r, d, mock.Client())
	if err == nil {
		t.Fatal("expected error when swarm task list is empty, got nil")
	}
	if !strings.Contains(err.Error(), "no tasks found") {
		t.Errorf("error should mention missing tasks, got: %v", err)
	}
	if d.Id() != "" {
		t.Errorf("expected empty ID after error, got %q", d.Id())
	}
}

// TestContainerExecCreate_Swarm_TasksHTTPError verifies an HTTP failure on the
// swarm task lookup propagates.
func TestContainerExecCreate_Swarm_TasksHTTPError(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/endpoints/1/docker/tasks", RespondString(
		http.StatusInternalServerError, "application/json", `{"message":"down"}`,
	))

	r := resourceContainerExec()
	d := r.TestResourceData()
	_ = d.Set("endpoint_id", 1)
	_ = d.Set("service_name", "mysvc")
	_ = d.Set("command", "ls")
	_ = d.Set("mode", "swarm")

	if err := rcCreate(r, d, mock.Client()); err == nil {
		t.Fatal("expected error on 500 from task list, got nil")
	}
}
