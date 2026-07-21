package internal

import (
	"net/http"
	"testing"
)

// =========================================================================
// Additional coverage (cov2) for resource_kubernetes_namespace_ingresscontrollers.go:
// the 404-clears-ID and non-404 error branches in Read, plus the error branch
// in the disable-via-PUT Delete.
// =========================================================================

// TestK8sNsIngressCtrlCov2_Read_NotFound verifies a 404 on the read GET clears
// the resource ID.
func TestK8sNsIngressCtrlCov2_Read_NotFound(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/kubernetes/1/namespaces/default/ingresscontrollers",
		RespondString(http.StatusNotFound, "", ""))

	r := resourceKubernetesNamespaceIngressControllers()
	d := r.TestResourceData()
	d.SetId("1:default")
	_ = d.Set("environment_id", 1)
	_ = d.Set("namespace", "default")

	if err := rcRead(r, d, mock.Client()); err != nil {
		t.Fatalf("Read on 404 should not error, got %v", err)
	}
	if d.Id() != "" {
		t.Errorf("expected ID cleared on 404, got %q", d.Id())
	}
}

// TestK8sNsIngressCtrlCov2_Read_HTTPError verifies a non-404 error status
// surfaces as an error.
func TestK8sNsIngressCtrlCov2_Read_HTTPError(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/kubernetes/1/namespaces/default/ingresscontrollers",
		RespondString(http.StatusInternalServerError, "application/json", `{"message":"boom"}`))

	r := resourceKubernetesNamespaceIngressControllers()
	d := r.TestResourceData()
	_ = d.Set("environment_id", 1)
	_ = d.Set("namespace", "default")

	if err := rcRead(r, d, mock.Client()); err == nil {
		t.Fatal("expected error on HTTP 500, got nil")
	}
}

// TestK8sNsIngressCtrlCov2_Delete_HTTPError verifies the disable-via-PUT Delete
// surfaces a non-2xx response as an error.
func TestK8sNsIngressCtrlCov2_Delete_HTTPError(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("PUT", "/kubernetes/1/namespaces/default/ingresscontrollers",
		RespondString(http.StatusInternalServerError, "application/json", `{"message":"boom"}`))

	r := resourceKubernetesNamespaceIngressControllers()
	d := r.TestResourceData()
	d.SetId("1:default")
	_ = d.Set("environment_id", 1)
	_ = d.Set("namespace", "default")
	_ = d.Set("controllers", []interface{}{
		map[string]interface{}{
			"name":         "nginx-controller",
			"class_name":   "nginx",
			"type":         "nginx",
			"availability": true,
			"used":         false,
			"new":          false,
		},
	})

	if err := rcDelete(r, d, mock.Client()); err == nil {
		t.Fatal("expected error on HTTP 500, got nil")
	}
}
