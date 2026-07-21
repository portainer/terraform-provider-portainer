package internal

import (
	"net/http"
	"testing"
)

// TestContainerExecCov3_Swarm_HappyPath drives execInSwarm end-to-end: task
// lookup, node inspection (for the agent-target hostname), exec creation and
// exec start. This exercises the swarm branch that the existing tests only
// touch on its early-error paths.
func TestContainerExecCov3_Swarm_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/endpoints/1/docker/tasks", RespondJSON(http.StatusOK, []map[string]interface{}{
		{
			"NodeID": "node1",
			"Status": map[string]interface{}{
				"ContainerStatus": map[string]interface{}{
					"ContainerID": "cont1",
				},
			},
		},
	}))
	mock.On("GET", "/endpoints/1/docker/nodes/node1", RespondJSON(http.StatusOK, map[string]interface{}{
		"Description": map[string]interface{}{
			"Hostname": "swarm-host-1",
		},
	}))
	mock.On("POST", "/endpoints/1/docker/containers/cont1/exec", RespondJSON(http.StatusCreated, map[string]interface{}{
		"Id": "exec-swarm",
	}))
	mock.On("POST", "/endpoints/1/docker/exec/exec-swarm/start", RespondString(
		http.StatusOK, "application/vnd.docker.raw-stream",
		"swarm-output",
	))

	r := resourceContainerExec()
	d := r.TestResourceData()
	_ = d.Set("endpoint_id", 1)
	_ = d.Set("service_name", "mysvc")
	_ = d.Set("user", "root:root")
	_ = d.Set("command", "echo hi")
	_ = d.Set("mode", "swarm")

	if err := rcCreate(r, d, mock.Client()); err != nil {
		t.Fatalf("Create (swarm) failed: %v", err)
	}

	if d.Id() != "exec-swarm" {
		t.Errorf("expected ID %q, got %q", "exec-swarm", d.Id())
	}
	if got := d.Get("output"); got != "swarm-output" {
		t.Errorf("output: got %v", got)
	}

	// The exec request must target the resolved node hostname.
	execReq := mock.FindRequest("POST", "/endpoints/1/docker/containers/cont1/exec")
	if execReq == nil {
		t.Fatal("expected exec POST")
	}
	if got := execReq.Headers.Get("X-PortainerAgent-Target"); got != "swarm-host-1" {
		t.Errorf("X-PortainerAgent-Target: expected %q, got %q", "swarm-host-1", got)
	}
}

// TestContainerExecCov3_Standalone_ExecDecodeError covers the decode-failure
// branch of execInStandalone: the container is found, but the exec endpoint
// returns a body that is not valid JSON.
func TestContainerExecCov3_Standalone_ExecDecodeError(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/endpoints/1/docker/containers/json", RespondJSON(http.StatusOK, []map[string]interface{}{
		{"Id": "abc123", "Names": []string{"/myapp"}},
	}))
	mock.On("POST", "/endpoints/1/docker/containers/abc123/exec", RespondString(
		http.StatusOK, "application/json",
		`not-json`,
	))

	r := resourceContainerExec()
	d := r.TestResourceData()
	_ = d.Set("endpoint_id", 1)
	_ = d.Set("service_name", "myapp")
	_ = d.Set("command", "ls")
	_ = d.Set("mode", "standalone")

	if err := rcCreate(r, d, mock.Client()); err == nil {
		t.Fatal("expected decode error from invalid exec response, got nil")
	}
}
