package internal

import (
	"context"
	"net/http"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

// =========================================================================
// Additional coverage (cov2) for resource_stack.go: error paths in
// findExistingStackByName / fetchSwarmID / create helpers, the Read stack-file
// fetch error, the readStackAccessControl restricted-ownership branch, and the
// import variant without an explicit method (3-part composite ID) plus the
// per-part invalid-ID guards.
// =========================================================================

// TestStackCov2_FindExistingStackByName_ListError covers the >= 400 branch of
// findExistingStackByName: GET /stacks fails, so Create aborts with an error
// before any create POST is sent.
func TestStackCov2_FindExistingStackByName_ListError(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/stacks", RespondString(
		http.StatusInternalServerError, "application/json", `{"message":"list boom"}`,
	))

	r := resourcePortainerStack()
	d := r.TestResourceData()
	_ = d.Set("deployment_type", "standalone")
	_ = d.Set("method", "string")
	_ = d.Set("name", "web")
	_ = d.Set("endpoint_id", 1)
	_ = d.Set("stack_file_content", "version: '3'")

	if err := rcCreate(r, d, mock.Client()); err == nil {
		t.Fatal("expected error when GET /stacks list fails, got nil")
	}
}

// TestStackCov2_FetchSwarmID_Error covers the fetchSwarmID error branch: a
// swarm deployment with empty swarm_id triggers fetchSwarmID, which returns a
// non-200 and aborts Create.
func TestStackCov2_FetchSwarmID_Error(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/endpoints/1/docker/swarm", RespondString(
		http.StatusInternalServerError, "application/json", `{"message":"no swarm"}`,
	))

	r := resourcePortainerStack()
	d := r.TestResourceData()
	_ = d.Set("deployment_type", "swarm")
	_ = d.Set("method", "string")
	_ = d.Set("name", "svc")
	_ = d.Set("endpoint_id", 1)
	_ = d.Set("stack_file_content", "version: '3'")
	// swarm_id intentionally empty -> fetchSwarmID runs and fails.

	if err := rcCreate(r, d, mock.Client()); err == nil {
		t.Fatal("expected error when fetchSwarmID fails, got nil")
	}
}

// TestStackCov2_KubernetesStringCreate_Error covers the non-200 branch of
// createStackK8sString.
func TestStackCov2_KubernetesStringCreate_Error(t *testing.T) {
	mock := NewMockServer(t)
	mockEmptyStackList(mock)

	mock.On("POST", "/stacks/create/kubernetes/string", RespondString(
		http.StatusBadRequest, "application/json", `{"message":"bad manifest"}`,
	))

	r := resourcePortainerStack()
	d := r.TestResourceData()
	_ = d.Set("deployment_type", "kubernetes")
	_ = d.Set("method", "string")
	_ = d.Set("name", "k8sapp")
	_ = d.Set("endpoint_id", 2)
	_ = d.Set("namespace", "default")
	_ = d.Set("stack_file_content", "apiVersion: v1")

	if err := rcCreate(r, d, mock.Client()); err == nil {
		t.Fatal("expected error on k8s string create 400, got nil")
	}
}

// TestStackCov2_KubernetesURLCreate_Error covers the non-200 branch of
// createStackK8sURL.
func TestStackCov2_KubernetesURLCreate_Error(t *testing.T) {
	mock := NewMockServer(t)
	mockEmptyStackList(mock)

	mock.On("POST", "/stacks/create/kubernetes/url", RespondString(
		http.StatusBadRequest, "application/json", `{"message":"bad url"}`,
	))

	r := resourcePortainerStack()
	d := r.TestResourceData()
	_ = d.Set("deployment_type", "kubernetes")
	_ = d.Set("method", "url")
	_ = d.Set("name", "k8surl")
	_ = d.Set("endpoint_id", 2)
	_ = d.Set("manifest_url", "https://example.com/manifest.yml")

	if err := rcCreate(r, d, mock.Client()); err == nil {
		t.Fatal("expected error on k8s url create 400, got nil")
	}
}

// TestStackCov2_RepositoryCreate_Error covers the non-200 branch of
// createStackStandaloneRepo.
func TestStackCov2_RepositoryCreate_Error(t *testing.T) {
	mock := NewMockServer(t)
	mockEmptyStackList(mock)

	mock.On("POST", "/stacks/create/standalone/repository", RespondString(
		http.StatusBadRequest, "application/json", `{"message":"bad repo"}`,
	))

	r := resourcePortainerStack()
	d := r.TestResourceData()
	_ = d.Set("deployment_type", "standalone")
	_ = d.Set("method", "repository")
	_ = d.Set("name", "gitstack")
	_ = d.Set("endpoint_id", 1)
	_ = d.Set("repository_url", "https://github.com/acme/app.git")

	if err := rcCreate(r, d, mock.Client()); err == nil {
		t.Fatal("expected error on repository create 400, got nil")
	}
}

// TestStackCov2_InvalidDeploymentType verifies an unknown deployment_type is
// rejected. (deployment_type has a validator at plan time, but the switch
// default is reachable from the handler directly with TestResourceData.)
func TestStackCov2_InvalidDeploymentType(t *testing.T) {
	mock := NewMockServer(t)
	mockEmptyStackList(mock)

	r := resourcePortainerStack()
	d := r.TestResourceData()
	_ = d.Set("deployment_type", "bogus")
	_ = d.Set("method", "string")
	_ = d.Set("name", "x")
	_ = d.Set("endpoint_id", 1)

	if err := rcCreate(r, d, mock.Client()); err == nil {
		t.Fatal("expected error for invalid deployment_type, got nil")
	}
}

// TestStackCov2_InvalidMethodStandalone verifies an unknown method for a
// standalone deployment hits the inner switch default.
func TestStackCov2_InvalidMethodStandalone(t *testing.T) {
	mock := NewMockServer(t)
	mockEmptyStackList(mock)

	r := resourcePortainerStack()
	d := r.TestResourceData()
	_ = d.Set("deployment_type", "standalone")
	_ = d.Set("method", "url") // url is invalid for standalone
	_ = d.Set("name", "x")
	_ = d.Set("endpoint_id", 1)

	if err := rcCreate(r, d, mock.Client()); err == nil {
		t.Fatal("expected error for invalid method on standalone, got nil")
	}
}

// TestStackCov2_Read_FileFetchError covers the stack-file fetch >= 400 branch
// in Read: the stack GET succeeds (non-repository) but GET /stacks/{id}/file
// returns an error.
func TestStackCov2_Read_FileFetchError(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/stacks/63", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id": 63, "Name": "app", "Status": 1, "Type": 2, "EndpointId": 1,
	}))
	mock.On("GET", "/stacks/63/file", RespondString(
		http.StatusInternalServerError, "application/json", `{"message":"file boom"}`,
	))

	r := resourcePortainerStack()
	d := r.TestResourceData()
	_ = d.Set("method", "string")
	d.SetId("63")

	if err := rcRead(r, d, mock.Client()); err == nil {
		t.Fatal("expected error when stack file fetch returns 500, got nil")
	}
}

// TestStackCov2_Read_RestrictedAccessControl covers the readStackAccessControl
// restricted branch (neither Public nor AdministratorsOnly) plus the team/user
// access flattening, exercised through Read when ResourceControl.Id is set.
func TestStackCov2_Read_RestrictedAccessControl(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/stacks/64", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id": 64, "Name": "app", "Status": 1, "Type": 2, "EndpointId": 1,
		"Portainer": map[string]interface{}{
			"ResourceControl": map[string]interface{}{"Id": 700},
		},
	}))
	mock.On("GET", "/stacks/64/file", RespondJSON(http.StatusOK, map[string]interface{}{
		"StackFileContent": "version: '3'",
	}))
	mock.On("GET", "/resource_controls/700", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id":                 700,
		"Public":             false,
		"AdministratorsOnly": false,
		"TeamAccesses":       []map[string]interface{}{{"TeamId": 12}},
		"UserAccesses":       []map[string]interface{}{{"UserId": 34}},
	}))

	r := resourcePortainerStack()
	d := r.TestResourceData()
	_ = d.Set("method", "string")
	d.SetId("64")

	if err := rcRead(r, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if got := d.Get("ownership"); got != "restricted" {
		t.Errorf("ownership: expected restricted, got %v", got)
	}
	if got := d.Get("resource_control_id"); got != 700 {
		t.Errorf("resource_control_id: expected 700, got %v", got)
	}
	teams := d.Get("authorized_teams").(*schema.Set).List()
	if len(teams) != 1 || teams[0].(int) != 12 {
		t.Errorf("authorized_teams: expected [12], got %v", teams)
	}
	users := d.Get("authorized_users").(*schema.Set).List()
	if len(users) != 1 || users[0].(int) != 34 {
		t.Errorf("authorized_users: expected [34], got %v", users)
	}
}

// TestStackCov2_Read_AccessControlFetchError covers the >= 400 branch of
// readStackAccessControl: the resource_controls GET fails, so Read surfaces an
// error.
func TestStackCov2_Read_AccessControlFetchError(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/stacks/65", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id": 65, "Name": "app", "Status": 1, "Type": 2, "EndpointId": 1,
		"Portainer": map[string]interface{}{
			"ResourceControl": map[string]interface{}{"Id": 800},
		},
	}))
	mock.On("GET", "/stacks/65/file", RespondJSON(http.StatusOK, map[string]interface{}{
		"StackFileContent": "version: '3'",
	}))
	mock.On("GET", "/resource_controls/800", RespondString(
		http.StatusInternalServerError, "application/json", `{"message":"rc boom"}`,
	))

	r := resourcePortainerStack()
	d := r.TestResourceData()
	_ = d.Set("method", "string")
	d.SetId("65")

	if err := rcRead(r, d, mock.Client()); err == nil {
		t.Fatal("expected error when readStackAccessControl GET returns 500, got nil")
	}
}

// TestStackCov2_Import_NoMethod covers the import path with the 3-part form
// "<endpoint>-<stack>-<deployment>" (no method segment), so the optional
// d.Set("method", ...) branch is skipped.
func TestStackCov2_Import_NoMethod(t *testing.T) {
	r := resourcePortainerStack()
	d := r.TestResourceData()
	d.SetId("7-21-swarm")

	out, err := r.Importer.StateContext(context.Background(), d, nil)
	if err != nil {
		t.Fatalf("import failed: %v", err)
	}
	rd := out[0]
	if rd.Id() != "21" {
		t.Errorf("expected stack ID 21, got %q", rd.Id())
	}
	if got := rd.Get("endpoint_id"); got != 7 {
		t.Errorf("endpoint_id: expected 7, got %v", got)
	}
	if got := rd.Get("deployment_type"); got != "swarm" {
		t.Errorf("deployment_type: expected swarm, got %v", got)
	}
}

// TestStackCov2_Import_InvalidEndpoint covers the non-numeric endpoint_id guard
// in the import state func.
func TestStackCov2_Import_InvalidEndpoint(t *testing.T) {
	r := resourcePortainerStack()
	d := r.TestResourceData()
	d.SetId("xx-21-swarm")

	if _, err := r.Importer.StateContext(context.Background(), d, nil); err == nil {
		t.Fatal("expected error for non-numeric endpoint_id in import ID, got nil")
	}
}

// TestStackCov2_Import_InvalidStackID covers the non-numeric stack_id guard in
// the import state func.
func TestStackCov2_Import_InvalidStackID(t *testing.T) {
	r := resourcePortainerStack()
	d := r.TestResourceData()
	d.SetId("7-yy-swarm")

	if _, err := r.Importer.StateContext(context.Background(), d, nil); err == nil {
		t.Fatal("expected error for non-numeric stack_id in import ID, got nil")
	}
}
