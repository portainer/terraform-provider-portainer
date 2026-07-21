package internal

import (
	"net/http"
	"testing"
)

// =========================================================================
// Additional coverage (cov4) for resource_kubernetes_namespace.go: the
// annotations loop in Create and Update (previous tests never set
// annotations), the licensed Update happy path with a resource quota, and the
// invalid-ID guard in Update.
// =========================================================================

// TestK8sNamespaceCov4_Create_WithAnnotations covers the annotations GetOk loop
// in Create by supplying an annotations map (unlicensed path).
func TestK8sNamespaceCov4_Create_WithAnnotations(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/licenses", RespondJSON(http.StatusOK, []map[string]interface{}{}))
	mock.On("POST", "/kubernetes/1/namespaces", RespondJSON(http.StatusOK, map[string]interface{}{}))
	mock.On("GET", "/kubernetes/1/namespaces/ann-ns", RespondJSON(http.StatusOK, map[string]interface{}{"Name": "ann-ns"}))

	r := resourceKubernetesNamespace()
	d := r.TestResourceData()
	_ = d.Set("environment_id", 1)
	_ = d.Set("name", "ann-ns")
	_ = d.Set("annotations", map[string]interface{}{"team": "platform"})

	if err := rcCreate(r, d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if d.Id() != "1:ann-ns" {
		t.Errorf("expected ID %q, got %q", "1:ann-ns", d.Id())
	}

	post := mock.FindRequest("POST", "/kubernetes/1/namespaces")
	if post == nil {
		t.Fatal("expected POST recorded")
	}
	var payload map[string]interface{}
	if err := post.DecodeJSON(&payload); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	anns, ok := payload["Annotations"].(map[string]interface{})
	if !ok || anns["team"] != "platform" {
		t.Errorf("expected annotation team=platform, got %v", payload["Annotations"])
	}
}

// TestK8sNamespaceCov4_Update_WithAnnotationsLicensed covers the annotations
// loop and the licensed resource-quota shape in Update (same name, so no ID
// change).
func TestK8sNamespaceCov4_Update_WithAnnotationsLicensed(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/licenses", RespondJSON(http.StatusOK, []map[string]interface{}{{"id": 1}}))
	mock.On("PUT", "/kubernetes/2/namespaces/team-a", RespondJSON(http.StatusOK, map[string]interface{}{}))
	mock.On("GET", "/kubernetes/2/namespaces/team-a", RespondJSON(http.StatusOK, map[string]interface{}{"Name": "team-a"}))

	r := resourceKubernetesNamespace()
	d := r.TestResourceData()
	d.SetId("2:team-a")
	_ = d.Set("environment_id", 2)
	_ = d.Set("name", "team-a")
	_ = d.Set("annotations", map[string]interface{}{"env": "prod"})
	_ = d.Set("resource_quota", map[string]interface{}{
		"cpu_limit":    "2",
		"memory_limit": "1Gi",
	})

	if err := rcUpdate(r, d, mock.Client()); err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	put := mock.FindRequest("PUT", "/kubernetes/2/namespaces/team-a")
	if put == nil {
		t.Fatal("expected PUT recorded")
	}
	var payload map[string]interface{}
	if err := put.DecodeJSON(&payload); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	anns, ok := payload["Annotations"].(map[string]interface{})
	if !ok || anns["env"] != "prod" {
		t.Errorf("expected annotation env=prod, got %v", payload["Annotations"])
	}
	rq, ok := payload["ResourceQuota"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected ResourceQuota map, got %T", payload["ResourceQuota"])
	}
	if rq["cpuLimit"] != "2" {
		t.Errorf("cpuLimit: expected 2, got %v", rq["cpuLimit"])
	}
	if rq["memoryLimit"] != "1Gi" {
		t.Errorf("memoryLimit: expected 1Gi, got %v", rq["memoryLimit"])
	}
}

// TestK8sNamespaceCov4_Update_InvalidIDFormat verifies Update errors when the
// ID lacks the envID:name separator. hasLicense is queried first (the missing
// /licenses route returns 404, treated as unlicensed) before the ID guard.
func TestK8sNamespaceCov4_Update_InvalidIDFormat(t *testing.T) {
	mock := NewMockServer(t)

	r := resourceKubernetesNamespace()
	d := r.TestResourceData()
	d.SetId("no-colon")
	_ = d.Set("environment_id", 1)
	_ = d.Set("name", "x")

	if err := rcUpdate(r, d, mock.Client()); err == nil {
		t.Fatal("expected error for malformed ID, got nil")
	}
}
