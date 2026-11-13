package internal

import (
	"context"
	"net/http"
	"strings"
	"testing"
)

// TestDataSourceGitopsWorkflowDeploymentResources_Read maps engine.Resource
// onto snake_case attributes, keeping a cluster-scoped resource's empty
// namespace.
func TestDataSourceGitopsWorkflowDeploymentResources_Read(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("GET", "/gitops/workflows/4/deployments/21/resources", RespondJSON(http.StatusOK, []map[string]interface{}{
		{"apiVersion": "apps/v1", "kind": "Deployment", "name": "web", "namespace": "web"},
		{"apiVersion": "v1", "kind": "Namespace", "name": "web"},
	}))

	r := dataSourceGitopsWorkflowDeploymentResources()
	d := r.TestResourceData()
	_ = d.Set("workflow_id", 4)
	_ = d.Set("deployment_id", 21)

	if diags := r.ReadContext(context.Background(), d, mock.Client()); diags.HasError() {
		t.Fatalf("Read failed: %v", diags)
	}
	if d.Id() != "4:21" {
		t.Errorf("id: got %q", d.Id())
	}
	resources := d.Get("resources").([]interface{})
	if len(resources) != 2 {
		t.Fatalf("expected two resources, got %d", len(resources))
	}
	first := resources[0].(map[string]interface{})
	if first["api_version"] != "apps/v1" || first["kind"] != "Deployment" || first["name"] != "web" || first["namespace"] != "web" {
		t.Errorf("resources[0]: %v", first)
	}
	if ns := resources[1].(map[string]interface{})["namespace"]; ns != "" {
		t.Errorf("resources[1].namespace: expected empty, got %v", ns)
	}
}

// TestDataSourceGitopsWorkflowDeploymentResources_OlderServer: the route is
// new in 3.0, so a 2.45 server's 404 names the requirement.
func TestDataSourceGitopsWorkflowDeploymentResources_OlderServer(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("GET", "/gitops/workflows/4/deployments/21/resources", RespondString(http.StatusNotFound, "text/plain", "404 page not found"))
	mock.On("GET", "/system/version", RespondJSON(http.StatusOK, map[string]interface{}{
		"ServerVersion": "2.45.3", "ServerEdition": "BE",
	}))

	r := dataSourceGitopsWorkflowDeploymentResources()
	d := r.TestResourceData()
	_ = d.Set("workflow_id", 4)
	_ = d.Set("deployment_id", 21)

	diags := r.ReadContext(context.Background(), d, mock.Client())
	if !diags.HasError() || !strings.Contains(diags[0].Summary, "requires Portainer Business Edition 3.0 or later") {
		t.Fatalf("expected the requirement to be named, got %v", diags)
	}
}
