package internal

import (
	"net/http"
	"testing"
)

const serviceManifestJSON = `{
  "apiVersion": "v1",
  "kind": "Service",
  "metadata": {"name": "websvc"},
  "spec": {"ports": [{"port": 80}]}
}`

// TestKubernetesServiceCreate_HappyPath verifies that Create POSTs to the
// services endpoint and builds the composite ID.
func TestKubernetesServiceCreate_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("POST", "/endpoints/2/kubernetes/api/v1/namespaces/prod/services", RespondJSON(http.StatusCreated, map[string]interface{}{
		"kind": "Service",
	}))

	r := resourceKubernetesService()
	d := r.TestResourceData()
	_ = d.Set("endpoint_id", 2)
	_ = d.Set("namespace", "prod")
	_ = d.Set("manifest", serviceManifestJSON)

	if err := rcCreate(r, d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if d.Id() != "2:prod:websvc" {
		t.Errorf("expected ID %q, got %q", "2:prod:websvc", d.Id())
	}

	post := mock.FindRequest("POST", "/endpoints/2/kubernetes/api/v1/namespaces/prod/services")
	if post == nil {
		t.Fatal("expected POST request to be recorded")
	}
	var payload map[string]interface{}
	if err := post.DecodeJSON(&payload); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if payload["kind"] != "Service" {
		t.Errorf("payload.kind: expected %q, got %v", "Service", payload["kind"])
	}
}

// TestKubernetesServiceCreate_MissingMetadataName verifies that the resource
// fails fast when the manifest omits metadata.name.
func TestKubernetesServiceCreate_MissingMetadataName(t *testing.T) {
	mock := NewMockServer(t)

	r := resourceKubernetesService()
	d := r.TestResourceData()
	_ = d.Set("endpoint_id", 1)
	_ = d.Set("namespace", "default")
	_ = d.Set("manifest", `{"kind":"Service","metadata":{}}`)

	if err := rcCreate(r, d, mock.Client()); err == nil {
		t.Fatal("expected error when metadata.name missing, got nil")
	}
	if d.Id() != "" {
		t.Errorf("expected empty ID after error, got %q", d.Id())
	}
}

// TestKubernetesServiceCreate_HTTPError verifies HTTP error surfaces.
func TestKubernetesServiceCreate_HTTPError(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("POST", "/endpoints/1/kubernetes/api/v1/namespaces/default/services", RespondString(
		http.StatusInternalServerError, "application/json",
		`{"message":"boom"}`,
	))

	r := resourceKubernetesService()
	d := r.TestResourceData()
	_ = d.Set("endpoint_id", 1)
	_ = d.Set("namespace", "default")
	_ = d.Set("manifest", serviceManifestJSON)

	if err := rcCreate(r, d, mock.Client()); err == nil {
		t.Fatal("expected error on HTTP 500, got nil")
	}
}

// TestKubernetesServiceDelete_HappyPath verifies DELETE is sent and ID cleared.
func TestKubernetesServiceDelete_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("DELETE", "/endpoints/2/kubernetes/api/v1/namespaces/prod/services/websvc", RespondString(
		http.StatusNoContent, "", "",
	))

	r := resourceKubernetesService()
	d := r.TestResourceData()
	d.SetId("2:prod:websvc")

	if err := rcDelete(r, d, mock.Client()); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	if mock.FindRequest("DELETE", "/endpoints/2/kubernetes/api/v1/namespaces/prod/services/websvc") == nil {
		t.Error("expected DELETE request to be recorded")
	}
	if d.Id() != "" {
		t.Errorf("expected ID cleared, got %q", d.Id())
	}
}

// TestKubernetesServiceParseID verifies ID parsing.
func TestKubernetesServiceParseID(t *testing.T) {
	endpointID, namespace, name := parseServiceID("3:ns:svc")
	if endpointID != 3 || namespace != "ns" || name != "svc" {
		t.Errorf("expected (3, ns, svc), got (%d, %q, %q)", endpointID, namespace, name)
	}
}
