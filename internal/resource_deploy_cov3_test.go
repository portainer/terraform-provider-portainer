package internal

import (
	"net/http"
	"testing"
)

// =========================================================================
// cov3 coverage for resource_deploy.go focused on the Docker Swarm branch of
// resourceDeployCreate, which resource_deploy_test.go leaves untouched (it only
// drives the standalone branch). force_update is left false throughout so the
// swarm branch's time.Sleep is never hit.
// =========================================================================

// TestDeployCov3_Swarm_HappyPath drives the full swarm branch: swarm detected,
// stack found by SwarmID filter, one matching service updated to a new tag, and
// the stack env var redeployed.
func TestDeployCov3_Swarm_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	// Swarm detection: 200 with an "ID" field => swarm.
	mock.On("GET", "/endpoints/1/docker/swarm", RespondJSON(http.StatusOK, map[string]interface{}{
		"ID": "swarm123",
	}))
	// Stack listing (query stripped by the mock).
	mock.On("GET", "/stacks", RespondJSON(http.StatusOK, []map[string]interface{}{
		{
			"Id":   7,
			"Name": "myapp",
			"Env": []map[string]interface{}{
				{"name": "APP_VERSION", "value": "1.0.0"},
			},
		},
	}))
	// Service listing for the stack.
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
	// Service update.
	mock.On("POST", "/endpoints/1/docker/services/myapp_web/update", RespondJSON(http.StatusOK, map[string]interface{}{}))
	// Stack file read + stack redeploy for the env-var update.
	mock.On("GET", "/stacks/7/file", RespondJSON(http.StatusOK, map[string]interface{}{
		"StackFileContent": "version: '3'\n",
	}))
	mock.On("PUT", "/stacks/7", RespondJSON(http.StatusOK, map[string]interface{}{}))

	r := resourceDeploy()
	d := r.TestResourceData()
	_ = d.Set("endpoint_id", 1)
	_ = d.Set("stack_name", "myapp")
	_ = d.Set("stack_env_var", "APP_VERSION")
	_ = d.Set("revision", "2.0.0")
	_ = d.Set("services_list", "web")
	_ = d.Set("update_revision", true)

	if err := rcCreate(r, d, mock.Client()); err != nil {
		t.Fatalf("swarm Create failed: %v", err)
	}
	if d.Id() == "" {
		t.Error("expected a non-empty deploy ID")
	}
	if mock.FindRequest("POST", "/endpoints/1/docker/services/myapp_web/update") == nil {
		t.Error("expected a swarm service update POST")
	}
	if mock.FindRequest("PUT", "/stacks/7") == nil {
		t.Error("expected a stack redeploy PUT for the env-var update")
	}
}

// TestDeployCov3_Swarm_StackNotFound covers the swarm branch error where the
// named stack is absent from the SwarmID-filtered stack listing.
func TestDeployCov3_Swarm_StackNotFound(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/endpoints/1/docker/swarm", RespondJSON(http.StatusOK, map[string]interface{}{
		"ID": "swarm123",
	}))
	mock.On("GET", "/stacks", RespondJSON(http.StatusOK, []map[string]interface{}{
		{"Id": 1, "Name": "somethingelse"},
	}))

	r := resourceDeploy()
	d := r.TestResourceData()
	_ = d.Set("endpoint_id", 1)
	_ = d.Set("stack_name", "myapp")
	_ = d.Set("stack_env_var", "APP_VERSION")
	_ = d.Set("revision", "2.0.0")
	_ = d.Set("services_list", "web")

	if err := rcCreate(r, d, mock.Client()); err == nil {
		t.Fatal("expected error when stack not found in swarm, got nil")
	}
}

// TestDeployCov3_Swarm_ServiceUpdateError covers the branch where a swarm
// service update returns a non-200 status, which is surfaced as an error.
func TestDeployCov3_Swarm_ServiceUpdateError(t *testing.T) {
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
	mock.On("POST", "/endpoints/1/docker/services/myapp_web/update", RespondString(
		http.StatusInternalServerError, "application/json", `{"message":"update boom"}`,
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
		t.Fatal("expected error when swarm service update returns 500, got nil")
	}
}

// TestDeployCov3_Swarm_AlreadyAtRevision covers the skip branch where the
// matching service is already at the target revision (no update POST is sent)
// and update_revision=false so no stack redeploy occurs.
func TestDeployCov3_Swarm_AlreadyAtRevision(t *testing.T) {
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
					"ContainerSpec": map[string]interface{}{"Image": "nginx:2.0.0"},
				},
				"Labels": map[string]interface{}{},
			},
			"Version": map[string]interface{}{"Index": 5},
		},
	}))

	r := resourceDeploy()
	d := r.TestResourceData()
	_ = d.Set("endpoint_id", 1)
	_ = d.Set("stack_name", "myapp")
	_ = d.Set("stack_env_var", "APP_VERSION")
	_ = d.Set("revision", "2.0.0")
	_ = d.Set("services_list", "web")
	_ = d.Set("update_revision", false)

	if err := rcCreate(r, d, mock.Client()); err != nil {
		t.Fatalf("swarm Create (already at revision) failed: %v", err)
	}
	if d.Id() == "" {
		t.Error("expected a deploy ID even when nothing was updated")
	}
	if mock.FindRequest("POST", "/endpoints/1/docker/services/myapp_web/update") != nil {
		t.Error("did not expect a service update POST when already at target revision")
	}
}
