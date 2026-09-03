package internal

import (
	"net/http"
	"strings"
	"testing"
)

// TestDataSourceKubernetesManifestDryRun_HappyPath verifies the data source
// POSTs the manifests to the 2.45 dry-run endpoint and maps every field of the
// per-document result.
func TestDataSourceKubernetesManifestDryRun_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("POST", "/kubernetes/7/manifests/dry_run", RespondJSON(http.StatusOK, map[string]interface{}{
		"results": []map[string]interface{}{
			{"kind": "ConfigMap", "name": "app-config", "namespace": "default", "documentIndex": 0, "status": "pass", "message": ""},
			{"kind": "Service", "name": "app", "namespace": "default", "documentIndex": 1, "status": "pass", "message": ""},
		},
	}))

	ds := dataSourceKubernetesManifestDryRun()
	d := ds.TestResourceData()
	_ = d.Set("environment_id", 7)
	_ = d.Set("namespace", "default")
	_ = d.Set("manifests", []interface{}{"kind: ConfigMap", "kind: Service"})

	if err := rcRead(ds, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	if got := d.Get("passed"); got != true {
		t.Errorf("passed: expected true, got %v", got)
	}
	if got := d.Get("failed_count"); got != 0 {
		t.Errorf("failed_count: expected 0, got %v", got)
	}

	results := d.Get("results").([]interface{})
	if len(results) != 2 {
		t.Fatalf("results: expected 2 entries, got %d", len(results))
	}
	first := results[0].(map[string]interface{})
	if first["kind"] != "ConfigMap" || first["name"] != "app-config" || first["status"] != "pass" {
		t.Errorf("results[0] mismatch: %v", first)
	}
	if first["document_index"] != 0 {
		t.Errorf("results[0].document_index: expected 0, got %v", first["document_index"])
	}

	post := mock.FindRequest("POST", "/kubernetes/7/manifests/dry_run")
	if post == nil {
		t.Fatal("expected POST to the dry-run endpoint")
	}
	var payload map[string]interface{}
	if err := post.DecodeJSON(&payload); err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	if payload["namespace"] != "default" {
		t.Errorf("payload.namespace: got %v", payload["namespace"])
	}
	manifests, ok := payload["manifests"].([]interface{})
	if !ok || len(manifests) != 2 {
		t.Fatalf("payload.manifests: expected 2 entries, got %v", payload["manifests"])
	}
}

// TestDataSourceKubernetesManifestDryRun_InvalidReported verifies that an
// invalid document is reported rather than raised, so the configuration can
// decide what to do with it.
func TestDataSourceKubernetesManifestDryRun_InvalidReported(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("POST", "/kubernetes/1/manifests/dry_run", RespondJSON(http.StatusOK, map[string]interface{}{
		"results": []map[string]interface{}{
			{"kind": "ConfigMap", "name": "ok", "namespace": "default", "documentIndex": 0, "status": "pass"},
			{"kind": "", "name": "", "namespace": "default", "documentIndex": 1, "status": "fail", "message": "error converting YAML to JSON"},
		},
	}))

	ds := dataSourceKubernetesManifestDryRun()
	d := ds.TestResourceData()
	_ = d.Set("environment_id", 1)
	_ = d.Set("manifests", []interface{}{"kind: ConfigMap", ":::not yaml"})

	if err := rcRead(ds, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if got := d.Get("passed"); got != false {
		t.Errorf("passed: expected false, got %v", got)
	}
	if got := d.Get("failed_count"); got != 1 {
		t.Errorf("failed_count: expected 1, got %v", got)
	}
	if d.Id() == "" {
		t.Error("expected the ID to be set so the failing outcome stays readable in state")
	}
}

// TestDataSourceKubernetesManifestDryRun_FailOnInvalid verifies the opt-in
// failure mode turns an invalid document into an error, and names the document
// that failed.
func TestDataSourceKubernetesManifestDryRun_FailOnInvalid(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("POST", "/kubernetes/1/manifests/dry_run", RespondJSON(http.StatusOK, map[string]interface{}{
		"results": []map[string]interface{}{
			{"kind": "Deployment", "name": "web", "namespace": "default", "documentIndex": 0, "status": "fail", "message": "spec.replicas: Invalid value"},
		},
	}))

	ds := dataSourceKubernetesManifestDryRun()
	d := ds.TestResourceData()
	_ = d.Set("environment_id", 1)
	_ = d.Set("manifests", []interface{}{"kind: Deployment"})
	_ = d.Set("fail_on_invalid", true)

	err := rcRead(ds, d, mock.Client())
	if err == nil {
		t.Fatal("expected Read to fail when fail_on_invalid is set and a document is invalid")
	}
	if !strings.Contains(err.Error(), "spec.replicas: Invalid value") {
		t.Errorf("error should carry the server's message, got: %v", err)
	}
}

// TestDataSourceKubernetesManifestDryRun_NamespaceOmitted verifies an unset
// namespace is left out of the payload entirely, so the documents are validated
// exactly as written.
func TestDataSourceKubernetesManifestDryRun_NamespaceOmitted(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("POST", "/kubernetes/2/manifests/dry_run", RespondJSON(http.StatusOK, map[string]interface{}{
		"results": []map[string]interface{}{},
	}))

	ds := dataSourceKubernetesManifestDryRun()
	d := ds.TestResourceData()
	_ = d.Set("environment_id", 2)
	_ = d.Set("manifests", []interface{}{"kind: ConfigMap"})

	if err := rcRead(ds, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	post := mock.FindRequest("POST", "/kubernetes/2/manifests/dry_run")
	var payload map[string]interface{}
	if err := post.DecodeJSON(&payload); err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	if _, present := payload["namespace"]; present {
		t.Errorf("payload should omit namespace when it is not configured, got %v", payload["namespace"])
	}
}
