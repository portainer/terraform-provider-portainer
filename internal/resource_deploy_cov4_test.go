package internal

import (
	"net/http"
	"testing"
)

// TestDeployCov4_Swarm_ForceUpdateAndWarnings covers two swarm-branch paths the
// existing cov3 suite leaves untouched: the force-update call (with wait=0 so no
// sleep occurs) and the "service update returned warnings" branch. update_revision
// is false so the stack env-var redeploy is skipped.
func TestDeployCov4_Swarm_ForceUpdateAndWarnings(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/endpoints/1/docker/swarm", RespondJSON(http.StatusOK, map[string]interface{}{
		"ID": "swarm123",
	}))
	mock.On("GET", "/stacks", RespondJSON(http.StatusOK, []map[string]interface{}{
		{"Id": 7, "Name": "myapp", "Env": []map[string]interface{}{}},
	}))
	mock.On("GET", "/endpoints/1/docker/services", RespondJSON(http.StatusOK, []map[string]interface{}{
		{
			"Spec": map[string]interface{}{
				"Name": "myapp_web",
				"TaskTemplate": map[string]interface{}{
					"ContainerSpec": map[string]interface{}{"Image": "nginx:1.0"},
				},
				"Labels": map[string]interface{}{},
			},
			"Version": map[string]interface{}{"Index": 5},
		},
	}))
	// Service update succeeds but returns a warning, exercising the warnings branch.
	mock.On("POST", "/endpoints/1/docker/services/myapp_web/update", RespondJSON(http.StatusOK, map[string]interface{}{
		"Warnings": "rescheduling delayed",
	}))
	// Force update endpoint.
	mock.On("PUT", "/endpoints/1/forceupdateservice", RespondJSON(http.StatusOK, map[string]interface{}{}))

	r := resourceDeploy()
	d := r.TestResourceData()
	_ = d.Set("endpoint_id", 1)
	_ = d.Set("stack_name", "myapp")
	_ = d.Set("stack_env_var", "APP_VERSION")
	_ = d.Set("revision", "2.0.0")
	_ = d.Set("services_list", "web")
	_ = d.Set("update_revision", false)
	_ = d.Set("force_update", true)
	_ = d.Set("wait", 0)

	if err := rcCreate(r, d, mock.Client()); err != nil {
		t.Fatalf("swarm force-update Create failed: %v", err)
	}
	if mock.FindRequest("POST", "/endpoints/1/docker/services/myapp_web/update") == nil {
		t.Error("expected a swarm service update POST")
	}
	if mock.FindRequest("PUT", "/endpoints/1/forceupdateservice") == nil {
		t.Error("expected a forceupdateservice PUT")
	}
}

// TestDeployCov4_Swarm_ForceUpdateFailsSoftly covers the branch where the
// force-update PUT returns a non-200: the failure is only logged to the output,
// not surfaced as an error.
func TestDeployCov4_Swarm_ForceUpdateFailsSoftly(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/endpoints/1/docker/swarm", RespondJSON(http.StatusOK, map[string]interface{}{
		"ID": "swarm123",
	}))
	mock.On("GET", "/stacks", RespondJSON(http.StatusOK, []map[string]interface{}{
		{"Id": 7, "Name": "myapp", "Env": []map[string]interface{}{}},
	}))
	mock.On("GET", "/endpoints/1/docker/services", RespondJSON(http.StatusOK, []map[string]interface{}{
		{
			"Spec": map[string]interface{}{
				"Name": "myapp_web",
				"TaskTemplate": map[string]interface{}{
					"ContainerSpec": map[string]interface{}{"Image": "nginx:1.0"},
				},
				"Labels": map[string]interface{}{},
			},
			"Version": map[string]interface{}{"Index": 5},
		},
	}))
	mock.On("POST", "/endpoints/1/docker/services/myapp_web/update", RespondJSON(http.StatusOK, map[string]interface{}{}))
	mock.On("PUT", "/endpoints/1/forceupdateservice", RespondString(
		http.StatusInternalServerError, "application/json", `{"message":"boom"}`,
	))

	r := resourceDeploy()
	d := r.TestResourceData()
	_ = d.Set("endpoint_id", 1)
	_ = d.Set("stack_name", "myapp")
	_ = d.Set("stack_env_var", "APP_VERSION")
	_ = d.Set("revision", "2.0.0")
	_ = d.Set("services_list", "web")
	_ = d.Set("update_revision", false)
	_ = d.Set("force_update", true)
	_ = d.Set("wait", 0)

	// A failing force update is soft: Create must still succeed.
	if err := rcCreate(r, d, mock.Client()); err != nil {
		t.Fatalf("Create should not fail on soft force-update error: %v", err)
	}
	if d.Id() == "" {
		t.Error("expected a deploy ID")
	}
}

// TestDeployCov4_Swarm_StackFileReadError covers the swarm env-var branch where
// reading the stack file returns a non-200 status, which is surfaced as an error.
func TestDeployCov4_Swarm_StackFileReadError(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/endpoints/1/docker/swarm", RespondJSON(http.StatusOK, map[string]interface{}{
		"ID": "swarm123",
	}))
	mock.On("GET", "/stacks", RespondJSON(http.StatusOK, []map[string]interface{}{
		{"Id": 7, "Name": "myapp", "Env": []map[string]interface{}{}},
	}))
	mock.On("GET", "/endpoints/1/docker/services", RespondJSON(http.StatusOK, []map[string]interface{}{}))
	mock.On("GET", "/stacks/7/file", RespondString(
		http.StatusInternalServerError, "application/json", `{"message":"boom"}`,
	))

	r := resourceDeploy()
	d := r.TestResourceData()
	_ = d.Set("endpoint_id", 1)
	_ = d.Set("stack_name", "myapp")
	_ = d.Set("stack_env_var", "APP_VERSION")
	_ = d.Set("revision", "2.0.0")
	_ = d.Set("services_list", "web")
	_ = d.Set("update_revision", true)

	if err := rcCreate(r, d, mock.Client()); err == nil {
		t.Fatal("expected error when swarm stack file read fails, got nil")
	}
}

// TestDeployCov4_Standalone_StackFileReadError covers the standalone env-var
// branch where reading the stack file returns a non-200 status.
func TestDeployCov4_Standalone_StackFileReadError(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/endpoints/1/docker/swarm", RespondString(
		http.StatusNotFound, "application/json", `{}`))
	mock.On("GET", "/stacks", RespondJSON(http.StatusOK, []map[string]interface{}{
		{"Id": 7, "Name": "myapp", "Env": []map[string]interface{}{}},
	}))
	mock.On("GET", "/stacks/7/file", RespondString(
		http.StatusInternalServerError, "application/json", `{"message":"boom"}`,
	))

	r := resourceDeploy()
	d := r.TestResourceData()
	_ = d.Set("endpoint_id", 1)
	_ = d.Set("stack_name", "myapp")
	_ = d.Set("stack_env_var", "APP_VERSION")
	_ = d.Set("revision", "2.0.0")
	_ = d.Set("services_list", "web")
	_ = d.Set("update_revision", true)

	if err := rcCreate(r, d, mock.Client()); err == nil {
		t.Fatal("expected error when standalone stack file read fails, got nil")
	}
}
