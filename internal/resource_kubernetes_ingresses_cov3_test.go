package internal

import (
	"context"
	"net/http"
	"testing"
)

// TestK8sIngressCov3_Import_HappyPath verifies the passthrough importer returns
// a single state with the composite ID preserved.
func TestK8sIngressCov3_Import_HappyPath(t *testing.T) {
	r := resourceKubernetesNamespaceIngress()
	d := r.TestResourceData()
	d.SetId("1:default:my-ingress")

	results, err := r.Importer.StateContext(context.Background(), d, nil)
	if err != nil {
		t.Fatalf("Import failed: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 imported state, got %d", len(results))
	}
	if results[0].Id() != "1:default:my-ingress" {
		t.Errorf("expected ID preserved, got %q", results[0].Id())
	}
}

// TestK8sIngressCov3_Read_InvalidIDFormat verifies Read errors when the ID does
// not split into three segments.
func TestK8sIngressCov3_Read_InvalidIDFormat(t *testing.T) {
	mock := NewMockServer(t)

	r := resourceKubernetesNamespaceIngress()
	d := r.TestResourceData()
	d.SetId("1:default")

	if err := rcRead(r, d, mock.Client()); err == nil {
		t.Fatal("expected error for malformed ID, got nil")
	}
}

// TestK8sIngressCov3_Read_InvalidEnvID verifies Read errors when the envID
// segment is not numeric (parses to zero).
func TestK8sIngressCov3_Read_InvalidEnvID(t *testing.T) {
	mock := NewMockServer(t)

	r := resourceKubernetesNamespaceIngress()
	d := r.TestResourceData()
	d.SetId("abc:default:my-ingress")

	if err := rcRead(r, d, mock.Client()); err == nil {
		t.Fatal("expected error for non-numeric envID, got nil")
	}
}

// TestK8sIngressCov3_Read_HappyPath verifies Read confirms existence via the
// networking.k8s.io proxy GET and repopulates identity fields.
func TestK8sIngressCov3_Read_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/endpoints/1/kubernetes/apis/networking.k8s.io/v1/namespaces/default/ingresses/my-ingress",
		RespondJSON(http.StatusOK, map[string]interface{}{"kind": "Ingress"}))

	r := resourceKubernetesNamespaceIngress()
	d := r.TestResourceData()
	d.SetId("1:default:my-ingress")

	if err := rcRead(r, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if got := d.Get("environment_id"); got != 1 {
		t.Errorf("environment_id: expected 1, got %v", got)
	}
	if got := d.Get("namespace"); got != "default" {
		t.Errorf("namespace: expected %q, got %v", "default", got)
	}
	if got := d.Get("name"); got != "my-ingress" {
		t.Errorf("name: expected %q, got %v", "my-ingress", got)
	}
}

// TestK8sIngressCov3_Read_HTTPError verifies a non-404 error on the existence
// GET surfaces as an error.
func TestK8sIngressCov3_Read_HTTPError(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/endpoints/1/kubernetes/apis/networking.k8s.io/v1/namespaces/default/ingresses/my-ingress",
		RespondString(http.StatusInternalServerError, "application/json", `{"message":"boom"}`))

	r := resourceKubernetesNamespaceIngress()
	d := r.TestResourceData()
	d.SetId("1:default:my-ingress")

	if err := rcRead(r, d, mock.Client()); err == nil {
		t.Fatal("expected error on HTTP 500, got nil")
	}
}

// TestK8sIngressCov3_Create_WithTLSAnnotationsLabels covers the annotations,
// labels and tls collection branches of createOrUpdateIngress.
func TestK8sIngressCov3_Create_WithTLSAnnotationsLabels(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("POST", "/kubernetes/1/namespaces/default/ingresses",
		RespondString(http.StatusOK, "application/json", `{}`))

	r := resourceKubernetesNamespaceIngress()
	d := r.TestResourceData()
	_ = d.Set("environment_id", 1)
	_ = d.Set("namespace", "default")
	_ = d.Set("name", "my-ingress")
	_ = d.Set("class_name", "nginx")
	_ = d.Set("annotations", map[string]interface{}{"kubernetes.io/ingress.class": "nginx"})
	_ = d.Set("labels", map[string]interface{}{"app": "web"})
	_ = d.Set("tls", []interface{}{
		map[string]interface{}{
			"hosts":       []interface{}{"example.com"},
			"secret_name": "tls-secret",
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
	if _, ok := payload["TLS"]; !ok {
		t.Error("expected TLS key in payload")
	}
	if _, ok := payload["Annotations"]; !ok {
		t.Error("expected Annotations key in payload")
	}
	if _, ok := payload["Labels"]; !ok {
		t.Error("expected Labels key in payload")
	}
}
