package internal

import (
	"encoding/json"
	"net/http"
	"testing"
)

// TestKubernetesApplicationCreate_HappyPath verifies that Create POSTs the
// parsed manifest (as JSON) to
// /endpoints/{envID}/kubernetes/apis/apps/v1/namespaces/{ns}/deployments and
// sets the composite ID "<envID>:<ns>:<name>" from metadata.name.
func TestKubernetesApplicationCreate_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("POST", "/endpoints/1/kubernetes/apis/apps/v1/namespaces/default/deployments",
		RespondJSON(http.StatusOK, map[string]interface{}{}))

	manifest := `apiVersion: apps/v1
kind: Deployment
metadata:
  name: my-app
  namespace: default
spec:
  replicas: 1
`

	r := resourceKubernetesApplication()
	d := r.TestResourceData()
	_ = d.Set("endpoint_id", 1)
	_ = d.Set("namespace", "default")
	_ = d.Set("manifest", manifest)

	if err := r.Create(d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if d.Id() != "1:default:my-app" {
		t.Errorf("expected ID %q, got %q", "1:default:my-app", d.Id())
	}

	post := mock.FindRequest("POST", "/endpoints/1/kubernetes/apis/apps/v1/namespaces/default/deployments")
	if post == nil {
		t.Fatal("expected POST recorded")
	}
	// Body is JSON-marshaled from parsed YAML.
	var payload map[string]interface{}
	if err := json.Unmarshal(post.Body, &payload); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if payload["kind"] != "Deployment" {
		t.Errorf("kind: expected Deployment, got %v", payload["kind"])
	}
	meta, ok := payload["metadata"].(map[string]interface{})
	if !ok {
		t.Fatalf("metadata not a map, got %T", payload["metadata"])
	}
	if meta["name"] != "my-app" {
		t.Errorf("metadata.name: expected my-app, got %v", meta["name"])
	}
	if ct := post.Headers.Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type: expected application/json, got %q", ct)
	}
}

// TestKubernetesApplicationCreate_InvalidManifest verifies that a manifest
// missing metadata.name surfaces an error.
func TestKubernetesApplicationCreate_InvalidManifest(t *testing.T) {
	mock := NewMockServer(t)

	r := resourceKubernetesApplication()
	d := r.TestResourceData()
	_ = d.Set("endpoint_id", 1)
	_ = d.Set("namespace", "default")
	// Manifest without metadata.name.
	_ = d.Set("manifest", `{"kind":"Deployment","metadata":{}}`)

	if err := r.Create(d, mock.Client()); err == nil {
		t.Fatal("expected error on manifest missing metadata.name, got nil")
	}
}

// TestKubernetesApplicationCreate_HTTPError verifies that 4xx/5xx propagates.
func TestKubernetesApplicationCreate_HTTPError(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("POST", "/endpoints/2/kubernetes/apis/apps/v1/namespaces/team/deployments",
		RespondString(http.StatusBadRequest, "application/json", `{"message":"invalid spec"}`))

	r := resourceKubernetesApplication()
	d := r.TestResourceData()
	_ = d.Set("endpoint_id", 2)
	_ = d.Set("namespace", "team")
	_ = d.Set("manifest", `{"kind":"Deployment","metadata":{"name":"bad"}}`)

	if err := r.Create(d, mock.Client()); err == nil {
		t.Fatal("expected error on HTTP 400, got nil")
	}
	if d.Id() != "" {
		t.Errorf("expected empty ID after error, got %q", d.Id())
	}
}

// TestKubernetesApplicationDelete_HappyPath verifies DELETE on the deployment
// endpoint clears the resource ID.
func TestKubernetesApplicationDelete_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("DELETE", "/endpoints/1/kubernetes/apis/apps/v1/namespaces/default/deployments/my-app",
		RespondJSON(http.StatusOK, map[string]interface{}{}))

	r := resourceKubernetesApplication()
	d := r.TestResourceData()
	d.SetId("1:default:my-app")

	if err := r.Delete(d, mock.Client()); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	if d.Id() != "" {
		t.Errorf("expected ID cleared, got %q", d.Id())
	}
	if del := mock.FindRequest("DELETE", "/endpoints/1/kubernetes/apis/apps/v1/namespaces/default/deployments/my-app"); del == nil {
		t.Fatal("expected DELETE request recorded")
	}
}

// TestKubernetesApplicationRead_Noop verifies that Read is a no-op (always nil).
func TestKubernetesApplicationRead_Noop(t *testing.T) {
	mock := NewMockServer(t)

	r := resourceKubernetesApplication()
	d := r.TestResourceData()
	d.SetId("1:default:my-app")

	if err := r.Read(d, mock.Client()); err != nil {
		t.Fatalf("Read should be a no-op, got error: %v", err)
	}
}
