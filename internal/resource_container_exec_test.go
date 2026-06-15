package internal

import (
	"net/http"
	"strings"
	"testing"
)

// TestContainerExecCreate_Standalone_HappyPath drives the standalone branch
// end-to-end: list containers by name, POST /exec to create the exec instance,
// POST /exec/{id}/start to run it, and capture the output into state.
func TestContainerExecCreate_Standalone_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/endpoints/1/docker/containers/json", RespondJSON(http.StatusOK, []map[string]interface{}{
		{"Id": "abc123", "Names": []string{"/myapp"}},
	}))
	mock.On("POST", "/endpoints/1/docker/containers/abc123/exec", RespondJSON(http.StatusCreated, map[string]interface{}{
		"Id": "exec-xyz",
	}))
	mock.On("POST", "/endpoints/1/docker/exec/exec-xyz/start", RespondString(
		http.StatusOK, "application/vnd.docker.raw-stream",
		"hello-world-output",
	))

	r := resourceContainerExec()
	d := r.TestResourceData()
	_ = d.Set("endpoint_id", 1)
	_ = d.Set("service_name", "myapp")
	_ = d.Set("user", "root:root")
	_ = d.Set("command", "echo hello")
	_ = d.Set("mode", "standalone")

	if err := rcCreate(r, d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	if d.Id() != "exec-xyz" {
		t.Errorf("expected ID %q, got %q", "exec-xyz", d.Id())
	}
	if got := d.Get("output"); got != "hello-world-output" {
		t.Errorf("output: expected captured stdout, got %v", got)
	}

	// Verify the exec payload carries command split into argv and the user.
	execReq := mock.FindRequest("POST", "/endpoints/1/docker/containers/abc123/exec")
	if execReq == nil {
		t.Fatal("expected POST /endpoints/1/docker/containers/abc123/exec")
	}
	var execPayload map[string]interface{}
	if err := execReq.DecodeJSON(&execPayload); err != nil {
		t.Fatalf("decode exec payload: %v", err)
	}
	if got := execPayload["User"]; got != "root:root" {
		t.Errorf("exec.User: got %v", got)
	}
	cmd, ok := execPayload["Cmd"].([]interface{})
	if !ok || len(cmd) != 2 || cmd[0] != "echo" || cmd[1] != "hello" {
		t.Errorf("exec.Cmd: expected [echo hello], got %v", execPayload["Cmd"])
	}
	if got := execPayload["AttachStdout"]; got != true {
		t.Errorf("exec.AttachStdout: expected true, got %v", got)
	}

	// The lookup query string must filter by the requested container name.
	listReq := mock.FindRequest("GET", "/endpoints/1/docker/containers/json")
	if listReq == nil {
		t.Fatal("expected GET to list containers")
	}
	if !strings.Contains(listReq.Query, "myapp") {
		t.Errorf("expected list query to filter by name containing %q, got %q", "myapp", listReq.Query)
	}
}

// TestContainerExecCreate_Standalone_ContainerNotFound verifies that when the
// container lookup returns an empty list, Create surfaces a "no container
// found" error without proceeding to the exec POST.
func TestContainerExecCreate_Standalone_ContainerNotFound(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/endpoints/1/docker/containers/json", RespondJSON(http.StatusOK, []map[string]interface{}{}))

	r := resourceContainerExec()
	d := r.TestResourceData()
	_ = d.Set("endpoint_id", 1)
	_ = d.Set("service_name", "ghost")
	_ = d.Set("command", "ls")
	_ = d.Set("mode", "standalone")

	err := rcCreate(r, d, mock.Client())
	if err == nil {
		t.Fatal("expected error when container list is empty")
	}
	if !strings.Contains(err.Error(), "no container found") {
		t.Errorf("error should mention missing container, got: %v", err)
	}
	if d.Id() != "" {
		t.Errorf("expected empty ID after error, got %q", d.Id())
	}
}

// TestContainerExecCreate_Standalone_ListHTTPError verifies that an HTTP
// failure on the initial container listing propagates and Create aborts.
func TestContainerExecCreate_Standalone_ListHTTPError(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/endpoints/1/docker/containers/json", RespondString(
		http.StatusInternalServerError, "application/json",
		`{"message":"docker daemon down"}`,
	))

	r := resourceContainerExec()
	d := r.TestResourceData()
	_ = d.Set("endpoint_id", 1)
	_ = d.Set("service_name", "myapp")
	_ = d.Set("command", "ls")
	_ = d.Set("mode", "standalone")

	err := rcCreate(r, d, mock.Client())
	if err == nil {
		t.Fatal("expected error on 500 from container list, got nil")
	}
	if d.Id() != "" {
		t.Errorf("expected empty ID after error, got %q", d.Id())
	}
	// The exec endpoint must not have been called.
	if got := mock.FindRequest("POST", "/endpoints/1/docker/containers/abc123/exec"); got != nil {
		t.Error("exec POST should not be sent when container listing fails")
	}
}

// TestContainerExecDelete_ClearsID verifies the trivial Delete contract:
// the resource is stateless on the server side, so Delete just clears the ID.
func TestContainerExecDelete_ClearsID(t *testing.T) {
	r := resourceContainerExec()
	d := r.TestResourceData()
	d.SetId("exec-xyz")

	if err := rcDelete(r, d, nil); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	if d.Id() != "" {
		t.Errorf("expected ID cleared after Delete, got %q", d.Id())
	}
}
