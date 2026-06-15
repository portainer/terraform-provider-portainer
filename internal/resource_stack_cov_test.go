package internal

import (
	"context"
	"net/http"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

// =========================================================================
// Additional coverage for resource_stack.go targeting paths not exercised by
// resource_stack_test.go: fetchSwarmID, swarm/k8s repository + k8s url create
// helpers, the create finalize PUT error, the Read non-404 HTTP error, the
// AutoUpdate/webhook read branches, the access-control side paths
// (updateStackAccessControl / readStackAccessControl / expandIntSet), the
// import state func, and the Delete generic-error branch.
// =========================================================================

// TestStackCreate_SwarmString_FetchesSwarmID covers the deployment_type=swarm
// branch where swarm_id is empty, forcing fetchSwarmID to call
// GET /endpoints/{id}/docker/swarm.
func TestStackCreate_SwarmString_FetchesSwarmID(t *testing.T) {
	mock := NewMockServer(t)
	mockEmptyStackList(mock)

	mock.On("GET", "/endpoints/1/docker/swarm", RespondJSON(http.StatusOK, map[string]interface{}{
		"ID": "swarm-fetched",
	}))
	mock.On("POST", "/stacks/create/swarm/string", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id": 40, "Name": "svc",
	}))
	mock.On("PUT", "/stacks/40", RespondJSON(http.StatusOK, map[string]interface{}{"Id": 40}))
	mock.On("GET", "/stacks/40", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id": 40, "Name": "svc", "Status": 1, "Type": 1, "SwarmId": "swarm-fetched", "EndpointId": 1,
	}))
	mock.On("GET", "/stacks/40/file", RespondJSON(http.StatusOK, map[string]interface{}{
		"StackFileContent": "version: '3'",
	}))

	r := resourcePortainerStack()
	d := r.TestResourceData()
	_ = d.Set("deployment_type", "swarm")
	_ = d.Set("method", "string")
	_ = d.Set("name", "svc")
	_ = d.Set("endpoint_id", 1)
	_ = d.Set("stack_file_content", "version: '3'")
	// swarm_id intentionally left empty -> fetchSwarmID runs.

	if err := rcCreate(r, d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if mock.FindRequest("GET", "/endpoints/1/docker/swarm") == nil {
		t.Error("expected fetchSwarmID call to GET /endpoints/1/docker/swarm")
	}
	if got := d.Get("swarm_id"); got != "swarm-fetched" {
		t.Errorf("swarm_id: expected fetched value, got %v", got)
	}
}

// TestStackCreate_SwarmRepository_HappyPath covers createStackSwarmRepo.
func TestStackCreate_SwarmRepository_HappyPath(t *testing.T) {
	mock := NewMockServer(t)
	mockEmptyStackList(mock)

	mock.On("POST", "/stacks/create/swarm/repository", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id": 41, "Name": "gitswarm",
	}))
	mock.On("GET", "/stacks/41", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id": 41, "Name": "gitswarm", "Status": 1, "Type": 1, "EndpointId": 1,
	}))

	r := resourcePortainerStack()
	d := r.TestResourceData()
	_ = d.Set("deployment_type", "swarm")
	_ = d.Set("method", "repository")
	_ = d.Set("name", "gitswarm")
	_ = d.Set("endpoint_id", 1)
	_ = d.Set("swarm_id", "swarm-x")
	_ = d.Set("repository_url", "https://github.com/acme/swarm.git")

	if err := rcCreate(r, d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	post := mock.FindRequest("POST", "/stacks/create/swarm/repository")
	if post == nil {
		t.Fatal("expected POST /stacks/create/swarm/repository")
	}
	var payload map[string]interface{}
	if err := post.DecodeJSON(&payload); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got := payload["swarmID"]; got != "swarm-x" {
		t.Errorf("payload.swarmID: expected swarm-x, got %v", got)
	}
	if got := payload["composeFile"]; got != "docker-compose.yml" {
		t.Errorf("payload.composeFile: expected default, got %v", got)
	}
}

// TestStackCreate_KubernetesRepository_HappyPath covers createStackK8sRepo.
func TestStackCreate_KubernetesRepository_HappyPath(t *testing.T) {
	mock := NewMockServer(t)
	mockEmptyStackList(mock)

	mock.On("POST", "/stacks/create/kubernetes/repository", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id": 42, "Name": "k8sgit",
	}))
	mock.On("GET", "/stacks/42", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id": 42, "Name": "k8sgit", "Status": 1, "Type": 3, "EndpointId": 2, "namespace": "default",
	}))

	r := resourcePortainerStack()
	d := r.TestResourceData()
	_ = d.Set("deployment_type", "kubernetes")
	_ = d.Set("method", "repository")
	_ = d.Set("name", "k8sgit")
	_ = d.Set("endpoint_id", 2)
	_ = d.Set("namespace", "default")
	_ = d.Set("repository_url", "https://github.com/acme/k8s.git")
	_ = d.Set("file_path_in_repository", "manifest.yml")

	if err := rcCreate(r, d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	post := mock.FindRequest("POST", "/stacks/create/kubernetes/repository")
	if post == nil {
		t.Fatal("expected POST /stacks/create/kubernetes/repository")
	}
	var payload map[string]interface{}
	if err := post.DecodeJSON(&payload); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got := payload["manifestFile"]; got != "manifest.yml" {
		t.Errorf("payload.manifestFile: expected manifest.yml, got %v", got)
	}
	if got := payload["stackName"]; got != "k8sgit" {
		t.Errorf("payload.stackName: expected k8sgit, got %v", got)
	}
}

// TestStackCreate_KubernetesURL_HappyPath covers createStackK8sURL.
func TestStackCreate_KubernetesURL_HappyPath(t *testing.T) {
	mock := NewMockServer(t)
	mockEmptyStackList(mock)

	mock.On("POST", "/stacks/create/kubernetes/url", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id": 43, "Name": "k8surl",
	}))
	mock.On("PUT", "/stacks/43", RespondJSON(http.StatusOK, map[string]interface{}{"Id": 43}))
	mock.On("GET", "/stacks/43", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id": 43, "Name": "k8surl", "Status": 1, "Type": 3, "EndpointId": 2, "namespace": "default",
	}))
	mock.On("GET", "/stacks/43/file", RespondJSON(http.StatusOK, map[string]interface{}{
		"StackFileContent": "apiVersion: v1",
	}))

	r := resourcePortainerStack()
	d := r.TestResourceData()
	_ = d.Set("deployment_type", "kubernetes")
	_ = d.Set("method", "url")
	_ = d.Set("name", "k8surl")
	_ = d.Set("endpoint_id", 2)
	_ = d.Set("namespace", "default")
	_ = d.Set("manifest_url", "https://example.com/manifest.yml")

	if err := rcCreate(r, d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	post := mock.FindRequest("POST", "/stacks/create/kubernetes/url")
	if post == nil {
		t.Fatal("expected POST /stacks/create/kubernetes/url")
	}
	var payload map[string]interface{}
	if err := post.DecodeJSON(&payload); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got := payload["manifestURL"]; got != "https://example.com/manifest.yml" {
		t.Errorf("payload.manifestURL: got %v", got)
	}
}

// TestStackCreate_FinalizePUTError verifies that a failure on the post-create
// finalize PUT (prune/webhook) propagates as an error.
func TestStackCreate_FinalizePUTError(t *testing.T) {
	mock := NewMockServer(t)
	mockEmptyStackList(mock)

	mock.On("POST", "/stacks/create/standalone/string", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id": 50, "Name": "web",
	}))
	mock.On("PUT", "/stacks/50", RespondString(
		http.StatusInternalServerError, "application/json", `{"message":"prune failed"}`,
	))

	r := resourcePortainerStack()
	d := r.TestResourceData()
	_ = d.Set("deployment_type", "standalone")
	_ = d.Set("method", "string")
	_ = d.Set("name", "web")
	_ = d.Set("endpoint_id", 1)
	_ = d.Set("stack_file_content", "version: '3'")

	if err := rcCreate(r, d, mock.Client()); err == nil {
		t.Fatal("expected error when finalize PUT fails, got nil")
	}
}

// TestStackRead_HTTPError verifies a non-404 error status on the stack GET is
// surfaced (>= 400 path, not the 404 drift path).
func TestStackRead_HTTPError(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/stacks/60", RespondString(
		http.StatusInternalServerError, "application/json", `{"message":"boom"}`,
	))

	r := resourcePortainerStack()
	d := r.TestResourceData()
	_ = d.Set("method", "string")
	d.SetId("60")

	if err := rcRead(r, d, mock.Client()); err == nil {
		t.Fatal("expected error on HTTP 500 read, got nil")
	}
}

// TestStackRead_AutoUpdateAndWebhook covers the AutoUpdate + webhook branches
// of Read (stack_webhook=true, pull_image and update_interval populated).
func TestStackRead_AutoUpdateAndWebhook(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/stacks/61", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id": 61, "Name": "auto", "Status": 1, "Type": 2, "EndpointId": 1,
		"AutoUpdate": map[string]interface{}{
			"Interval":       "5m",
			"Webhook":        "wh-uuid",
			"ForcePullImage": true,
		},
	}))
	mock.On("GET", "/stacks/61/file", RespondJSON(http.StatusOK, map[string]interface{}{
		"StackFileContent": "version: '3'",
	}))

	r := resourcePortainerStack()
	d := r.TestResourceData()
	_ = d.Set("method", "string")
	d.SetId("61")

	if err := rcRead(r, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if got := d.Get("stack_webhook"); got != true {
		t.Errorf("stack_webhook: expected true, got %v", got)
	}
	if got := d.Get("webhook_id"); got != "wh-uuid" {
		t.Errorf("webhook_id: expected wh-uuid, got %v", got)
	}
	if got := d.Get("update_interval"); got != "5m" {
		t.Errorf("update_interval: expected 5m, got %v", got)
	}
	if got := d.Get("pull_image"); got != true {
		t.Errorf("pull_image: expected true, got %v", got)
	}
}

// TestStackRead_HelmConfig covers the helmConfig read branch.
func TestStackRead_HelmConfig(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/stacks/62", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id": 62, "Name": "helm", "Status": 1, "Type": 3, "EndpointId": 2,
		"helmConfig": map[string]interface{}{
			"chartPath":   "charts/app",
			"valuesFiles": []string{"values-prod.yaml"},
		},
	}))

	r := resourcePortainerStack()
	d := r.TestResourceData()
	_ = d.Set("method", "repository")
	d.SetId("62")

	if err := rcRead(r, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if got := d.Get("helm_chart_path"); got != "charts/app" {
		t.Errorf("helm_chart_path: expected charts/app, got %v", got)
	}
}

// TestStackDelete_GenericError verifies a non-retryable, non-2xx, non-404
// status on delete surfaces an error.
func TestStackDelete_GenericError(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("DELETE", "/stacks/70", RespondString(
		http.StatusBadRequest, "application/json", `{"message":"cannot delete"}`,
	))

	r := resourcePortainerStack()
	d := r.TestResourceData()
	d.SetId("70")
	_ = d.Set("endpoint_id", 1)

	if err := rcDelete(r, d, mock.Client()); err == nil {
		t.Fatal("expected error on HTTP 400 delete, got nil")
	}
}

// TestStackImport_ParsesCompositeID exercises the Importer state func with the
// "<endpoint>-<stack>-<deployment>-<method>" form.
func TestStackImport_ParsesCompositeID(t *testing.T) {
	r := resourcePortainerStack()
	d := r.TestResourceData()
	d.SetId("3-15-standalone-string")

	out, err := r.Importer.StateContext(context.Background(), d, nil)
	if err != nil {
		t.Fatalf("import failed: %v", err)
	}
	if len(out) != 1 {
		t.Fatalf("expected 1 resource data, got %d", len(out))
	}
	rd := out[0]
	if rd.Id() != "15" {
		t.Errorf("expected stack ID 15, got %q", rd.Id())
	}
	if got := rd.Get("endpoint_id"); got != 3 {
		t.Errorf("endpoint_id: expected 3, got %v", got)
	}
	if got := rd.Get("deployment_type"); got != "standalone" {
		t.Errorf("deployment_type: expected standalone, got %v", got)
	}
	if got := rd.Get("method"); got != "string" {
		t.Errorf("method: expected string, got %v", got)
	}
}

// TestStackImport_InvalidID verifies the import guard rejects malformed IDs.
func TestStackImport_InvalidID(t *testing.T) {
	r := resourcePortainerStack()
	d := r.TestResourceData()
	d.SetId("bad-id")

	if _, err := r.Importer.StateContext(context.Background(), d, nil); err == nil {
		t.Fatal("expected error for malformed import ID, got nil")
	}
}

// TestExpandIntSet covers the expandIntSet helper directly using a real
// schema.Set built from the authorized_users attribute.
func TestExpandIntSet(t *testing.T) {
	r := resourcePortainerStack()
	d := r.TestResourceData()
	_ = d.Set("authorized_users", []interface{}{1, 2, 3})
	set := d.Get("authorized_users").(*schema.Set)
	got := expandIntSet(set)
	if len(got) != 3 {
		t.Fatalf("expected 3 ints, got %d (%v)", len(got), got)
	}
	sum := 0
	for _, v := range got {
		sum += v
	}
	if sum != 6 {
		t.Errorf("expected sum 6 from {1,2,3}, got %d", sum)
	}
}

// TestStackCreate_WithOwnership_UpdatesAccessControl covers the
// updateStackAccessControl + readStackAccessControl side paths that fire only
// when ownership is set. These go through client.DoRequest, so the resource
// looks up the resource control via /resource_controls and PUTs an update.
func TestStackCreate_WithOwnership_UpdatesAccessControl(t *testing.T) {
	mock := NewMockServer(t)
	mockEmptyStackList(mock)

	mock.On("POST", "/stacks/create/standalone/string", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id": 80, "Name": "owned",
	}))
	mock.On("PUT", "/stacks/80", RespondJSON(http.StatusOK, map[string]interface{}{"Id": 80}))

	// updateStackAccessControl -> lookupResourceControlID(client, 6, "80")
	// performs GET /stacks/80 and reads ResourceControl.Id. The same handler
	// also serves the later Read (which decodes Portainer.ResourceControl.Id),
	// so the response carries both shapes.
	mock.On("GET", "/stacks/80", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id": 80, "Name": "owned", "Status": 1, "Type": 2, "EndpointId": 1,
		"ResourceControl": map[string]interface{}{"Id": 500},
		"Portainer": map[string]interface{}{
			"ResourceControl": map[string]interface{}{"Id": 500},
		},
	}))
	// The PUT to update the resource control.
	mock.On("PUT", "/resource_controls/500", RespondJSON(http.StatusOK, map[string]interface{}{"Id": 500}))
	mock.On("GET", "/stacks/80/file", RespondJSON(http.StatusOK, map[string]interface{}{
		"StackFileContent": "version: '3'",
	}))
	// readStackAccessControl GETs the resource control by id.
	mock.On("GET", "/resource_controls/500", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id": 500, "Public": true,
	}))

	r := resourcePortainerStack()
	d := r.TestResourceData()
	_ = d.Set("deployment_type", "standalone")
	_ = d.Set("method", "string")
	_ = d.Set("name", "owned")
	_ = d.Set("endpoint_id", 1)
	_ = d.Set("stack_file_content", "version: '3'")
	_ = d.Set("ownership", "public")

	if err := rcCreate(r, d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if mock.FindRequest("PUT", "/resource_controls/500") == nil {
		t.Error("expected PUT /resource_controls/500 (access control update)")
	}
	if got := d.Get("ownership"); got != "public" {
		t.Errorf("ownership: expected public after readStackAccessControl, got %v", got)
	}
}
