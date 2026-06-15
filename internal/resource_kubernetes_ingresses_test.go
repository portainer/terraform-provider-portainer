package internal

import (
	"net/http"
	"testing"
)

// TestKubernetesIngressCreate_HappyPath verifies POST /kubernetes/{id}/namespaces/{ns}/ingresses
// and ID "<envID>:<ns>:<name>".
func TestKubernetesIngressCreate_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("POST", "/kubernetes/1/namespaces/default/ingresses",
		RespondString(http.StatusOK, "application/json", `{}`))

	r := resourceKubernetesNamespaceIngress()
	d := r.TestResourceData()
	_ = d.Set("environment_id", 1)
	_ = d.Set("namespace", "default")
	_ = d.Set("name", "my-ingress")
	_ = d.Set("class_name", "nginx")
	_ = d.Set("hosts", []interface{}{"example.com"})
	_ = d.Set("paths", []interface{}{
		map[string]interface{}{
			"host":         "example.com",
			"path":         "/",
			"path_type":    "Prefix",
			"port":         80,
			"service_name": "web",
		},
	})

	if err := rcCreate(r, d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if d.Id() != "1:default:my-ingress" {
		t.Errorf("expected ID %q, got %q", "1:default:my-ingress", d.Id())
	}

	post := mock.FindRequest("POST", "/kubernetes/1/namespaces/default/ingresses")
	if post == nil {
		t.Fatal("expected POST request to be recorded")
	}
	var payload map[string]interface{}
	if err := post.DecodeJSON(&payload); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if payload["Name"] != "my-ingress" {
		t.Errorf("payload.Name: expected %q, got %v", "my-ingress", payload["Name"])
	}
	if payload["Namespace"] != "default" {
		t.Errorf("payload.Namespace: expected %q, got %v", "default", payload["Namespace"])
	}
	if payload["ClassName"] != "nginx" {
		t.Errorf("payload.ClassName: expected %q, got %v", "nginx", payload["ClassName"])
	}
}

// TestKubernetesIngressCreate_HTTPError verifies HTTP error surfaces.
func TestKubernetesIngressCreate_HTTPError(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("POST", "/kubernetes/1/namespaces/default/ingresses",
		RespondString(http.StatusInternalServerError, "application/json", `{"message":"boom"}`))

	r := resourceKubernetesNamespaceIngress()
	d := r.TestResourceData()
	_ = d.Set("environment_id", 1)
	_ = d.Set("namespace", "default")
	_ = d.Set("name", "my-ingress")

	if err := rcCreate(r, d, mock.Client()); err == nil {
		t.Fatal("expected error on HTTP 500, got nil")
	}
}

// TestKubernetesIngressUpdate_HappyPath verifies PUT is used by Update.
func TestKubernetesIngressUpdate_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("PUT", "/kubernetes/1/namespaces/default/ingresses",
		RespondString(http.StatusOK, "application/json", `{}`))

	r := resourceKubernetesNamespaceIngress()
	d := r.TestResourceData()
	_ = d.Set("environment_id", 1)
	_ = d.Set("namespace", "default")
	_ = d.Set("name", "my-ingress")
	_ = d.Set("class_name", "nginx")

	if err := rcUpdate(r, d, mock.Client()); err != nil {
		t.Fatalf("Update failed: %v", err)
	}
	if mock.FindRequest("PUT", "/kubernetes/1/namespaces/default/ingresses") == nil {
		t.Error("expected PUT request to be recorded")
	}
}

// TestKubernetesIngressDelete_NoOp verifies Delete is a no-op (API not yet supported)
// and does not error or send any request.
func TestKubernetesIngressDelete_NoOp(t *testing.T) {
	mock := NewMockServer(t)

	r := resourceKubernetesNamespaceIngress()
	d := r.TestResourceData()
	d.SetId("1:default:my-ingress")

	if err := rcDelete(r, d, mock.Client()); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	if reqs := mock.Requests(); len(reqs) != 0 {
		t.Errorf("expected no requests for no-op Delete, got %d", len(reqs))
	}
}
