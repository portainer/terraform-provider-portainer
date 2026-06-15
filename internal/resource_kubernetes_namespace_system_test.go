package internal

import (
	"net/http"
	"testing"
)

// TestKubernetesNamespaceSystemCreate_HappyPath verifies that Create PUTs to
// /kubernetes/{envID}/namespaces/{ns}/system with {"system": true} and sets
// the composite ID.
func TestKubernetesNamespaceSystemCreate_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("PUT", "/kubernetes/1/namespaces/kube-system/system", RespondJSON(http.StatusOK, map[string]interface{}{}))

	r := resourceKubernetesNamespaceSystem()
	d := r.TestResourceData()
	_ = d.Set("environment_id", 1)
	_ = d.Set("namespace", "kube-system")
	_ = d.Set("system", true)

	if err := rcCreate(r, d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if d.Id() != "1:kube-system" {
		t.Errorf("expected ID %q, got %q", "1:kube-system", d.Id())
	}

	put := mock.FindRequest("PUT", "/kubernetes/1/namespaces/kube-system/system")
	if put == nil {
		t.Fatal("expected PUT request to be recorded")
	}
	var payload map[string]interface{}
	if err := put.DecodeJSON(&payload); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if payload["system"] != true {
		t.Errorf("payload.system: expected true, got %v", payload["system"])
	}
}

// TestKubernetesNamespaceSystemUpdate_HappyPath verifies Update uses the same PUT.
func TestKubernetesNamespaceSystemUpdate_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("PUT", "/kubernetes/1/namespaces/myns/system", RespondJSON(http.StatusOK, map[string]interface{}{}))

	r := resourceKubernetesNamespaceSystem()
	d := r.TestResourceData()
	d.SetId("1:myns")
	_ = d.Set("environment_id", 1)
	_ = d.Set("namespace", "myns")
	_ = d.Set("system", false)

	if err := rcUpdate(r, d, mock.Client()); err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	put := mock.FindRequest("PUT", "/kubernetes/1/namespaces/myns/system")
	if put == nil {
		t.Fatal("expected PUT request to be recorded")
	}
	var payload map[string]interface{}
	if err := put.DecodeJSON(&payload); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if payload["system"] != false {
		t.Errorf("payload.system: expected false, got %v", payload["system"])
	}
}

// TestKubernetesNamespaceSystemDelete_ForcesSystemFalse verifies that Delete
// flips system to false and re-issues the PUT (Unset implementation).
func TestKubernetesNamespaceSystemDelete_ForcesSystemFalse(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("PUT", "/kubernetes/1/namespaces/kube-system/system", RespondJSON(http.StatusOK, map[string]interface{}{}))

	r := resourceKubernetesNamespaceSystem()
	d := r.TestResourceData()
	d.SetId("1:kube-system")
	_ = d.Set("environment_id", 1)
	_ = d.Set("namespace", "kube-system")
	_ = d.Set("system", true)

	if err := rcDelete(r, d, mock.Client()); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	put := mock.FindRequest("PUT", "/kubernetes/1/namespaces/kube-system/system")
	if put == nil {
		t.Fatal("expected PUT request to be recorded")
	}
	var payload map[string]interface{}
	if err := put.DecodeJSON(&payload); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if payload["system"] != false {
		t.Errorf("expected Delete to send system=false, got %v", payload["system"])
	}
}

// TestKubernetesNamespaceSystemCreate_HTTPError verifies HTTP error surfaces.
func TestKubernetesNamespaceSystemCreate_HTTPError(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("PUT", "/kubernetes/1/namespaces/badns/system", RespondString(
		http.StatusBadRequest, "application/json", `{"message":"bad"}`,
	))

	r := resourceKubernetesNamespaceSystem()
	d := r.TestResourceData()
	_ = d.Set("environment_id", 1)
	_ = d.Set("namespace", "badns")
	_ = d.Set("system", true)

	if err := rcCreate(r, d, mock.Client()); err == nil {
		t.Fatal("expected error on HTTP 400, got nil")
	}
	if d.Id() != "" {
		t.Errorf("expected empty ID after error, got %q", d.Id())
	}
}
