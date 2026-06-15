package internal

import (
	"net/http"
	"testing"
)

const secretManifestJSON = `{
  "apiVersion": "v1",
  "kind": "Secret",
  "type": "Opaque",
  "metadata": {"name": "mysecret"},
  "data": {"password": "cGFzcw=="}
}`

// TestKubernetesSecretCreate_HappyPath verifies that Create POSTs to the
// secrets endpoint and builds the composite ID.
func TestKubernetesSecretCreate_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("POST", "/endpoints/2/kubernetes/api/v1/namespaces/prod/secrets", RespondJSON(http.StatusCreated, map[string]interface{}{
		"kind": "Secret",
	}))

	r := resourceKubernetesSecrets()
	d := r.TestResourceData()
	_ = d.Set("endpoint_id", 2)
	_ = d.Set("namespace", "prod")
	_ = d.Set("manifest", secretManifestJSON)

	if err := rcCreate(r, d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if d.Id() != "2:prod:mysecret" {
		t.Errorf("expected ID %q, got %q", "2:prod:mysecret", d.Id())
	}

	post := mock.FindRequest("POST", "/endpoints/2/kubernetes/api/v1/namespaces/prod/secrets")
	if post == nil {
		t.Fatal("expected POST request to be recorded")
	}
	var payload map[string]interface{}
	if err := post.DecodeJSON(&payload); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if payload["kind"] != "Secret" {
		t.Errorf("payload.kind: expected %q, got %v", "Secret", payload["kind"])
	}
	if payload["type"] != "Opaque" {
		t.Errorf("payload.type: expected %q, got %v", "Opaque", payload["type"])
	}
}

// TestKubernetesSecretCreate_MissingMetadataName verifies the resource fails fast
// when the manifest omits metadata.name.
func TestKubernetesSecretCreate_MissingMetadataName(t *testing.T) {
	mock := NewMockServer(t)

	r := resourceKubernetesSecrets()
	d := r.TestResourceData()
	_ = d.Set("endpoint_id", 1)
	_ = d.Set("namespace", "default")
	_ = d.Set("manifest", `{"kind":"Secret","metadata":{}}`)

	err := rcCreate(r, d, mock.Client())
	if err == nil {
		t.Fatal("expected error when metadata.name missing, got nil")
	}
	if d.Id() != "" {
		t.Errorf("expected empty ID after error, got %q", d.Id())
	}
}

// TestKubernetesSecretCreate_HTTPError verifies HTTP error surfaces.
func TestKubernetesSecretCreate_HTTPError(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("POST", "/endpoints/1/kubernetes/api/v1/namespaces/default/secrets", RespondString(
		http.StatusInternalServerError, "application/json",
		`{"message":"boom"}`,
	))

	r := resourceKubernetesSecrets()
	d := r.TestResourceData()
	_ = d.Set("endpoint_id", 1)
	_ = d.Set("namespace", "default")
	_ = d.Set("manifest", secretManifestJSON)

	err := rcCreate(r, d, mock.Client())
	if err == nil {
		t.Fatal("expected error on HTTP 500, got nil")
	}
}

// TestKubernetesSecretDelete_HappyPath verifies DELETE is sent and ID cleared.
func TestKubernetesSecretDelete_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("DELETE", "/endpoints/2/kubernetes/api/v1/namespaces/prod/secrets/mysecret", RespondString(
		http.StatusNoContent, "", "",
	))

	r := resourceKubernetesSecrets()
	d := r.TestResourceData()
	d.SetId("2:prod:mysecret")

	if err := rcDelete(r, d, mock.Client()); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	if mock.FindRequest("DELETE", "/endpoints/2/kubernetes/api/v1/namespaces/prod/secrets/mysecret") == nil {
		t.Error("expected DELETE request to be recorded")
	}
	if d.Id() != "" {
		t.Errorf("expected ID cleared, got %q", d.Id())
	}
}

// TestKubernetesSecretParseID verifies ID parsing.
func TestKubernetesSecretParseID(t *testing.T) {
	endpointID, namespace, name := parseSecretsID("3:ns:sec")
	if endpointID != 3 || namespace != "ns" || name != "sec" {
		t.Errorf("expected (3, ns, sec), got (%d, %q, %q)", endpointID, namespace, name)
	}
}
