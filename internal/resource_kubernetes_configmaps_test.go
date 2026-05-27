package internal

import (
	"net/http"
	"testing"
)

const configMapManifestJSON = `{
  "apiVersion": "v1",
  "kind": "ConfigMap",
  "metadata": {"name": "myconfig"},
  "data": {"key1": "value1"}
}`

// TestKubernetesConfigMapsCreate_HappyPath verifies that Create POSTs the
// manifest as JSON to the namespaces/configmaps endpoint and builds the
// composite ID "<endpointID>:<namespace>:<name>".
func TestKubernetesConfigMapsCreate_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("POST", "/endpoints/1/kubernetes/api/v1/namespaces/default/configmaps", RespondJSON(http.StatusCreated, map[string]interface{}{
		"kind": "ConfigMap",
	}))

	r := resourceKubernetesConfigMaps()
	d := r.TestResourceData()
	_ = d.Set("endpoint_id", 1)
	_ = d.Set("namespace", "default")
	_ = d.Set("manifest", configMapManifestJSON)

	if err := r.Create(d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	if d.Id() != "1:default:myconfig" {
		t.Errorf("expected ID %q, got %q", "1:default:myconfig", d.Id())
	}

	post := mock.FindRequest("POST", "/endpoints/1/kubernetes/api/v1/namespaces/default/configmaps")
	if post == nil {
		t.Fatal("expected POST request to be recorded")
	}
	var payload map[string]interface{}
	if err := post.DecodeJSON(&payload); err != nil {
		t.Fatalf("failed to decode JSON body: %v", err)
	}
	if payload["kind"] != "ConfigMap" {
		t.Errorf("payload.kind: expected %q, got %v", "ConfigMap", payload["kind"])
	}
	meta, ok := payload["metadata"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected metadata to be a map, got %T", payload["metadata"])
	}
	if meta["name"] != "myconfig" {
		t.Errorf("metadata.name: expected %q, got %v", "myconfig", meta["name"])
	}
}

// TestKubernetesConfigMapsCreate_InvalidManifest verifies that invalid JSON/YAML
// returns an error and the ID is left unset.
func TestKubernetesConfigMapsCreate_InvalidManifest(t *testing.T) {
	mock := NewMockServer(t)

	r := resourceKubernetesConfigMaps()
	d := r.TestResourceData()
	_ = d.Set("endpoint_id", 1)
	_ = d.Set("namespace", "default")
	_ = d.Set("manifest", "this: is: not: valid: yaml: [")

	err := r.Create(d, mock.Client())
	if err == nil {
		t.Fatal("expected error for invalid manifest, got nil")
	}
	if d.Id() != "" {
		t.Errorf("expected empty ID after error, got %q", d.Id())
	}
}

// TestKubernetesConfigMapsCreate_HTTPError verifies HTTP error surfaces.
func TestKubernetesConfigMapsCreate_HTTPError(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("POST", "/endpoints/1/kubernetes/api/v1/namespaces/default/configmaps", RespondString(
		http.StatusConflict, "application/json",
		`{"message":"already exists"}`,
	))

	r := resourceKubernetesConfigMaps()
	d := r.TestResourceData()
	_ = d.Set("endpoint_id", 1)
	_ = d.Set("namespace", "default")
	_ = d.Set("manifest", configMapManifestJSON)

	err := r.Create(d, mock.Client())
	if err == nil {
		t.Fatal("expected error on HTTP 409, got nil")
	}
	if d.Id() != "" {
		t.Errorf("expected empty ID after error, got %q", d.Id())
	}
}

// TestKubernetesConfigMapsDelete_HappyPath verifies DELETE is sent and ID cleared.
func TestKubernetesConfigMapsDelete_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("DELETE", "/endpoints/1/kubernetes/api/v1/namespaces/default/configmaps/myconfig", RespondString(
		http.StatusOK, "", "",
	))

	r := resourceKubernetesConfigMaps()
	d := r.TestResourceData()
	d.SetId("1:default:myconfig")

	if err := r.Delete(d, mock.Client()); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	if mock.FindRequest("DELETE", "/endpoints/1/kubernetes/api/v1/namespaces/default/configmaps/myconfig") == nil {
		t.Error("expected DELETE request to be recorded")
	}
	if d.Id() != "" {
		t.Errorf("expected ID cleared after Delete, got %q", d.Id())
	}
}

// TestKubernetesConfigMapsParseID verifies ID round-trip parsing.
func TestKubernetesConfigMapsParseID(t *testing.T) {
	endpointID, namespace, name := parseConfigMapsID("42:prod:my-cm")
	if endpointID != 42 || namespace != "prod" || name != "my-cm" {
		t.Errorf("expected (42, prod, my-cm), got (%d, %q, %q)", endpointID, namespace, name)
	}
	// malformed
	endpointID, namespace, name = parseConfigMapsID("bad")
	if endpointID != 0 || namespace != "" || name != "" {
		t.Errorf("expected zero values on malformed ID, got (%d, %q, %q)", endpointID, namespace, name)
	}
}
