package internal

import (
	"net/http"
	"testing"
)

// TestKubernetesNamespaceCreate_HappyPath_Unlicensed verifies Create POSTs to
// /kubernetes/{envID}/namespaces with the unlicensed resource quota shape and
// sets ID "<envID>:<name>". A /licenses GET precedes Create.
func TestKubernetesNamespaceCreate_HappyPath_Unlicensed(t *testing.T) {
	mock := NewMockServer(t)

	// Unlicensed: /licenses returns empty list.
	mock.On("GET", "/licenses", RespondJSON(http.StatusOK, []map[string]interface{}{}))
	mock.On("POST", "/kubernetes/1/namespaces", RespondJSON(http.StatusOK, map[string]interface{}{}))

	r := resourceKubernetesNamespace()
	d := r.TestResourceData()
	_ = d.Set("environment_id", 1)
	_ = d.Set("name", "my-ns")
	_ = d.Set("owner", "admin")
	_ = d.Set("resource_quota", map[string]interface{}{
		"cpu":    "1",
		"memory": "512Mi",
	})

	if err := r.Create(d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if d.Id() != "1:my-ns" {
		t.Errorf("expected ID %q, got %q", "1:my-ns", d.Id())
	}

	post := mock.FindRequest("POST", "/kubernetes/1/namespaces")
	if post == nil {
		t.Fatal("expected POST /kubernetes/1/namespaces")
	}
	var payload map[string]interface{}
	if err := post.DecodeJSON(&payload); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if payload["Name"] != "my-ns" {
		t.Errorf("payload.Name: expected %q, got %v", "my-ns", payload["Name"])
	}
	if payload["Owner"] != "admin" {
		t.Errorf("payload.Owner: expected %q, got %v", "admin", payload["Owner"])
	}
	rq, ok := payload["ResourceQuota"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected ResourceQuota map, got %T", payload["ResourceQuota"])
	}
	if rq["enabled"] != true {
		t.Errorf("ResourceQuota.enabled: expected true, got %v", rq["enabled"])
	}
	if rq["cpu"] != "1" {
		t.Errorf("ResourceQuota.cpu: expected %q, got %v", "1", rq["cpu"])
	}
	if rq["memory"] != "512Mi" {
		t.Errorf("ResourceQuota.memory: expected %q, got %v", "512Mi", rq["memory"])
	}
}

// TestKubernetesNamespaceCreate_HappyPath_Licensed verifies licensed code path
// uses cpuRequest/cpuLimit/memoryRequest/memoryLimit fields.
func TestKubernetesNamespaceCreate_HappyPath_Licensed(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/licenses", RespondJSON(http.StatusOK, []map[string]interface{}{
		{"id": 1, "company": "ACME"},
	}))
	mock.On("POST", "/kubernetes/2/namespaces", RespondJSON(http.StatusOK, map[string]interface{}{}))

	r := resourceKubernetesNamespace()
	d := r.TestResourceData()
	_ = d.Set("environment_id", 2)
	_ = d.Set("name", "team-a")
	_ = d.Set("resource_quota", map[string]interface{}{
		"cpu_request":    "500m",
		"cpu_limit":      "2",
		"memory_request": "256Mi",
		"memory_limit":   "1Gi",
	})

	if err := r.Create(d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if d.Id() != "2:team-a" {
		t.Errorf("expected ID %q, got %q", "2:team-a", d.Id())
	}

	post := mock.FindRequest("POST", "/kubernetes/2/namespaces")
	if post == nil {
		t.Fatal("expected POST /kubernetes/2/namespaces")
	}
	var payload map[string]interface{}
	if err := post.DecodeJSON(&payload); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	rq, ok := payload["ResourceQuota"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected ResourceQuota map, got %T", payload["ResourceQuota"])
	}
	if rq["cpuRequest"] != "500m" {
		t.Errorf("cpuRequest: expected 500m, got %v", rq["cpuRequest"])
	}
	if rq["cpuLimit"] != "2" {
		t.Errorf("cpuLimit: expected 2, got %v", rq["cpuLimit"])
	}
	if rq["memoryRequest"] != "256Mi" {
		t.Errorf("memoryRequest: expected 256Mi, got %v", rq["memoryRequest"])
	}
	if rq["memoryLimit"] != "1Gi" {
		t.Errorf("memoryLimit: expected 1Gi, got %v", rq["memoryLimit"])
	}
}

// TestKubernetesNamespaceCreate_HTTPError verifies HTTP error surfaces.
func TestKubernetesNamespaceCreate_HTTPError(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/licenses", RespondJSON(http.StatusOK, []map[string]interface{}{}))
	mock.On("POST", "/kubernetes/1/namespaces", RespondString(
		http.StatusConflict, "application/json", `{"message":"already exists"}`,
	))

	r := resourceKubernetesNamespace()
	d := r.TestResourceData()
	_ = d.Set("environment_id", 1)
	_ = d.Set("name", "dup")

	if err := r.Create(d, mock.Client()); err == nil {
		t.Fatal("expected error on HTTP 409, got nil")
	}
}

// TestKubernetesNamespaceUpdate_HappyPath verifies PUT to
// /kubernetes/{envID}/namespaces/{oldName}.
func TestKubernetesNamespaceUpdate_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/licenses", RespondJSON(http.StatusOK, []map[string]interface{}{}))
	mock.On("PUT", "/kubernetes/1/namespaces/my-ns", RespondJSON(http.StatusOK, map[string]interface{}{}))

	r := resourceKubernetesNamespace()
	d := r.TestResourceData()
	d.SetId("1:my-ns")
	_ = d.Set("environment_id", 1)
	_ = d.Set("name", "my-ns")
	_ = d.Set("owner", "ops")

	if err := r.Update(d, mock.Client()); err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	put := mock.FindRequest("PUT", "/kubernetes/1/namespaces/my-ns")
	if put == nil {
		t.Fatal("expected PUT request to be recorded")
	}
	var payload map[string]interface{}
	if err := put.DecodeJSON(&payload); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if payload["Owner"] != "ops" {
		t.Errorf("payload.Owner: expected %q, got %v", "ops", payload["Owner"])
	}
}

// TestKubernetesNamespaceDelete_HappyPath verifies DELETE sends the body
// {"Name": <name>} to /kubernetes/{envID}/namespaces.
func TestKubernetesNamespaceDelete_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("DELETE", "/kubernetes/1/namespaces", RespondJSON(http.StatusOK, map[string]interface{}{}))

	r := resourceKubernetesNamespace()
	d := r.TestResourceData()
	d.SetId("1:my-ns")

	if err := r.Delete(d, mock.Client()); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	del := mock.FindRequest("DELETE", "/kubernetes/1/namespaces")
	if del == nil {
		t.Fatal("expected DELETE request to be recorded")
	}
	var payload map[string]string
	if err := del.DecodeJSON(&payload); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if payload["Name"] != "my-ns" {
		t.Errorf("payload.Name: expected %q, got %v", "my-ns", payload["Name"])
	}
	if d.Id() != "" {
		t.Errorf("expected ID cleared, got %q", d.Id())
	}
}
