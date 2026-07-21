package internal

import (
	"net/http"
	"testing"
)

// =========================================================================
// Additional coverage (cov2) for resource_kubernetes_ingresscontrollers.go:
// the non-404 error branch in Read and the error branch in the
// disable-via-PUT Delete.
// =========================================================================

// TestK8sIngressCtrlCov2_Read_HTTPError verifies a non-404 error status on the
// read GET surfaces as an error.
func TestK8sIngressCtrlCov2_Read_HTTPError(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/kubernetes/1/ingresscontrollers",
		RespondString(http.StatusInternalServerError, "application/json", `{"message":"boom"}`))

	r := resourceKubernetesIngressControllers()
	d := r.TestResourceData()
	_ = d.Set("environment_id", 1)

	if err := rcRead(r, d, mock.Client()); err == nil {
		t.Fatal("expected error on HTTP 500, got nil")
	}
}

// TestK8sIngressCtrlCov2_Delete_HTTPError verifies the disable-via-PUT Delete
// surfaces a non-2xx response as an error.
func TestK8sIngressCtrlCov2_Delete_HTTPError(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("PUT", "/kubernetes/1/ingresscontrollers",
		RespondString(http.StatusInternalServerError, "application/json", `{"message":"boom"}`))

	r := resourceKubernetesIngressControllers()
	d := r.TestResourceData()
	d.SetId("1")
	_ = d.Set("environment_id", 1)
	_ = d.Set("controllers", []interface{}{
		map[string]interface{}{
			"availability": true,
			"class_name":   "nginx",
			"name":         "nginx-controller",
			"new":          false,
			"type":         "nginx",
			"used":         false,
		},
	})

	if err := rcDelete(r, d, mock.Client()); err == nil {
		t.Fatal("expected error on HTTP 500, got nil")
	}
}
