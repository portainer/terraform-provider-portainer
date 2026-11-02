package internal

import (
	"net/http"
	"strings"
	"testing"
)

// TestContainerExecCreate_Swarm_TaskWithoutContainer is the regression test for
// a provider crash: a task that is not scheduled yet has no NodeID and no
// ContainerStatus, and the unchecked type assertions panicked on it.
func TestContainerExecCreate_Swarm_TaskWithoutContainer(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("GET", "/endpoints/1/docker/tasks", RespondJSON(http.StatusOK, []map[string]interface{}{
		{"ID": "task1", "Status": map[string]interface{}{"State": "pending"}},
	}))

	r := resourceContainerExec()
	d := r.TestResourceData()
	_ = d.Set("endpoint_id", 1)
	_ = d.Set("service_name", "mysvc")
	_ = d.Set("command", "ls")
	_ = d.Set("mode", "swarm")

	err := rcCreate(r, d, mock.Client())
	if err == nil || !strings.Contains(err.Error(), "has no container yet") {
		t.Fatalf("expected a no-container error, got %v", err)
	}
}

// TestContainerExecCreate_Standalone_ContainerWithoutID covers a container
// entry without an Id, which used to panic on the type assertion.
func TestContainerExecCreate_Standalone_ContainerWithoutID(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("GET", "/endpoints/1/docker/containers/json", RespondJSON(http.StatusOK, []map[string]interface{}{
		{"Names": []string{"/web"}},
	}))

	r := resourceContainerExec()
	d := r.TestResourceData()
	_ = d.Set("endpoint_id", 1)
	_ = d.Set("service_name", "web")
	_ = d.Set("command", "ls")
	_ = d.Set("mode", "standalone")

	err := rcCreate(r, d, mock.Client())
	if err == nil || !strings.Contains(err.Error(), "has no ID") {
		t.Fatalf("expected a missing-ID error, got %v", err)
	}
}
