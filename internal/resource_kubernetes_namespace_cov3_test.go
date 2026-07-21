package internal

import (
	"context"
	"net/http"
	"testing"
)

// TestK8sNamespaceCov3_Import_HappyPath verifies the passthrough importer
// returns a single state with the composite ID preserved.
func TestK8sNamespaceCov3_Import_HappyPath(t *testing.T) {
	r := resourceKubernetesNamespace()
	d := r.TestResourceData()
	d.SetId("1:my-ns")

	results, err := r.Importer.StateContext(context.Background(), d, nil)
	if err != nil {
		t.Fatalf("Import failed: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 imported state, got %d", len(results))
	}
	if results[0].Id() != "1:my-ns" {
		t.Errorf("expected ID preserved, got %q", results[0].Id())
	}
}

// TestK8sNamespaceCov3_Read_InvalidIDFormat verifies Read errors when the ID
// does not contain the envID:name separator.
func TestK8sNamespaceCov3_Read_InvalidIDFormat(t *testing.T) {
	mock := NewMockServer(t)

	r := resourceKubernetesNamespace()
	d := r.TestResourceData()
	d.SetId("no-colon")

	if err := rcRead(r, d, mock.Client()); err == nil {
		t.Fatal("expected error for malformed ID, got nil")
	}
}

// TestK8sNamespaceCov3_Read_InvalidEnvID verifies Read errors when the envID
// segment is not numeric.
func TestK8sNamespaceCov3_Read_InvalidEnvID(t *testing.T) {
	mock := NewMockServer(t)

	r := resourceKubernetesNamespace()
	d := r.TestResourceData()
	d.SetId("abc:my-ns")

	if err := rcRead(r, d, mock.Client()); err == nil {
		t.Fatal("expected error for non-numeric envID, got nil")
	}
}

// TestK8sNamespaceCov3_Read_HTTPError verifies a non-404 error on the read GET
// surfaces as an error.
func TestK8sNamespaceCov3_Read_HTTPError(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/kubernetes/1/namespaces/my-ns",
		RespondString(http.StatusInternalServerError, "application/json", `{"message":"boom"}`))

	r := resourceKubernetesNamespace()
	d := r.TestResourceData()
	d.SetId("1:my-ns")

	if err := rcRead(r, d, mock.Client()); err == nil {
		t.Fatal("expected error on HTTP 500, got nil")
	}
}

// TestK8sNamespaceCov3_Read_QuotaAndOwner verifies Read maps the K8s
// ResourceQuota spec.hard keys back to the schema keys and records the owner.
func TestK8sNamespaceCov3_Read_QuotaAndOwner(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/kubernetes/1/namespaces/my-ns", RespondJSON(http.StatusOK, map[string]interface{}{
		"Name":           "my-ns",
		"NamespaceOwner": "ops",
		"ResourceQuota": map[string]interface{}{
			"spec": map[string]interface{}{
				"hard": map[string]interface{}{
					"limits.cpu":      "2",
					"limits.memory":   "1Gi",
					"requests.cpu":    "500m",
					"requests.memory": "256Mi",
				},
			},
		},
	}))

	r := resourceKubernetesNamespace()
	d := r.TestResourceData()
	d.SetId("1:my-ns")

	if err := rcRead(r, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if got := d.Get("owner"); got != "ops" {
		t.Errorf("owner: expected %q, got %v", "ops", got)
	}
	quota := d.Get("resource_quota").(map[string]interface{})
	if quota["cpu_limit"] != "2" {
		t.Errorf("cpu_limit: expected %q, got %v", "2", quota["cpu_limit"])
	}
	if quota["memory_request"] != "256Mi" {
		t.Errorf("memory_request: expected %q, got %v", "256Mi", quota["memory_request"])
	}
}

// TestK8sNamespaceCov3_Create_LicenseCheckError verifies that a license lookup
// returning undecodable JSON aborts Create with an error.
func TestK8sNamespaceCov3_Create_LicenseCheckError(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/licenses", RespondString(http.StatusOK, "application/json", "this is not json"))

	r := resourceKubernetesNamespace()
	d := r.TestResourceData()
	_ = d.Set("environment_id", 1)
	_ = d.Set("name", "my-ns")

	if err := rcCreate(r, d, mock.Client()); err == nil {
		t.Fatal("expected error when license check fails to decode, got nil")
	}
}

// TestK8sNamespaceCov3_Update_NameChange verifies Update PUTs against the old
// name and refreshes the ID to the new name.
func TestK8sNamespaceCov3_Update_NameChange(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/licenses", RespondJSON(http.StatusOK, []map[string]interface{}{}))
	mock.On("PUT", "/kubernetes/1/namespaces/old-ns", RespondJSON(http.StatusOK, map[string]interface{}{}))
	mock.On("GET", "/kubernetes/1/namespaces/new-ns", RespondJSON(http.StatusOK, map[string]interface{}{"Name": "new-ns"}))

	r := resourceKubernetesNamespace()
	d := r.TestResourceData()
	d.SetId("1:old-ns")
	_ = d.Set("environment_id", 1)
	_ = d.Set("name", "new-ns")

	if err := rcUpdate(r, d, mock.Client()); err != nil {
		t.Fatalf("Update failed: %v", err)
	}
	if mock.FindRequest("PUT", "/kubernetes/1/namespaces/old-ns") == nil {
		t.Error("expected PUT to old namespace name")
	}
	if d.Id() != "1:new-ns" {
		t.Errorf("expected ID updated to %q, got %q", "1:new-ns", d.Id())
	}
}

// TestK8sNamespaceCov3_Update_HTTPError verifies a non-2xx PUT surfaces an error.
func TestK8sNamespaceCov3_Update_HTTPError(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/licenses", RespondJSON(http.StatusOK, []map[string]interface{}{}))
	mock.On("PUT", "/kubernetes/1/namespaces/my-ns",
		RespondString(http.StatusInternalServerError, "application/json", `{"message":"boom"}`))

	r := resourceKubernetesNamespace()
	d := r.TestResourceData()
	d.SetId("1:my-ns")
	_ = d.Set("environment_id", 1)
	_ = d.Set("name", "my-ns")

	if err := rcUpdate(r, d, mock.Client()); err == nil {
		t.Fatal("expected error on HTTP 500, got nil")
	}
}

// TestK8sNamespaceCov3_Delete_InvalidID verifies Delete errors before any HTTP
// call when the ID lacks the envID:name separator.
func TestK8sNamespaceCov3_Delete_InvalidID(t *testing.T) {
	mock := NewMockServer(t)

	r := resourceKubernetesNamespace()
	d := r.TestResourceData()
	d.SetId("no-colon")

	if err := rcDelete(r, d, mock.Client()); err == nil {
		t.Fatal("expected error for malformed ID, got nil")
	}
	if reqs := mock.Requests(); len(reqs) != 0 {
		t.Errorf("expected no HTTP calls, got %d", len(reqs))
	}
}

// TestK8sNamespaceCov3_Delete_HTTPError verifies a non-2xx DELETE surfaces an error.
func TestK8sNamespaceCov3_Delete_HTTPError(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("DELETE", "/kubernetes/1/namespaces",
		RespondString(http.StatusInternalServerError, "application/json", `{"message":"boom"}`))

	r := resourceKubernetesNamespace()
	d := r.TestResourceData()
	d.SetId("1:my-ns")

	if err := rcDelete(r, d, mock.Client()); err == nil {
		t.Fatal("expected error on HTTP 500, got nil")
	}
}
