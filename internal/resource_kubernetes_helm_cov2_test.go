package internal

import (
	"net/http"
	"testing"
)

// =========================================================================
// Additional coverage (cov2) for resource_kubernetes_helm.go targeting the
// ID-parsing guards and the non-404 error branches in Read and Delete that the
// base test file does not exercise.
// =========================================================================

// TestK8sHelmCov2_Read_InvalidIDFormat verifies Read errors before any HTTP
// call when the ID lacks the "envID:namespace:release" separators.
func TestK8sHelmCov2_Read_InvalidIDFormat(t *testing.T) {
	mock := NewMockServer(t)

	r := resourceKubernetesHelm()
	d := r.TestResourceData()
	d.SetId("no-colons")

	if err := rcRead(r, d, mock.Client()); err == nil {
		t.Fatal("expected error for malformed ID, got nil")
	}
	if reqs := mock.Requests(); len(reqs) != 0 {
		t.Errorf("expected no HTTP calls, got %d", len(reqs))
	}
}

// TestK8sHelmCov2_Read_ZeroEnvID verifies Read errors when the envID segment
// parses to zero (the second guard clause).
func TestK8sHelmCov2_Read_ZeroEnvID(t *testing.T) {
	mock := NewMockServer(t)

	r := resourceKubernetesHelm()
	d := r.TestResourceData()
	d.SetId("0:default:rel")

	if err := rcRead(r, d, mock.Client()); err == nil {
		t.Fatal("expected error for zero envID, got nil")
	}
}

// TestK8sHelmCov2_Read_HTTPError verifies a non-404 error status on the read
// GET surfaces as an error.
func TestK8sHelmCov2_Read_HTTPError(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/endpoints/1/kubernetes/helm/r1",
		RespondString(http.StatusInternalServerError, "application/json", `{"message":"boom"}`))

	r := resourceKubernetesHelm()
	d := r.TestResourceData()
	d.SetId("1:default:r1")

	if err := rcRead(r, d, mock.Client()); err == nil {
		t.Fatal("expected error on HTTP 500, got nil")
	}
}

// TestK8sHelmCov2_Delete_InvalidIDFormat verifies Delete errors before any HTTP
// call when the ID lacks the expected separators.
func TestK8sHelmCov2_Delete_InvalidIDFormat(t *testing.T) {
	mock := NewMockServer(t)

	r := resourceKubernetesHelm()
	d := r.TestResourceData()
	d.SetId("no-colons")

	if err := rcDelete(r, d, mock.Client()); err == nil {
		t.Fatal("expected error for malformed ID, got nil")
	}
	if reqs := mock.Requests(); len(reqs) != 0 {
		t.Errorf("expected no HTTP calls, got %d", len(reqs))
	}
}

// TestK8sHelmCov2_Delete_HTTPError verifies a non-204 DELETE response surfaces
// as an error.
func TestK8sHelmCov2_Delete_HTTPError(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("DELETE", "/endpoints/1/kubernetes/helm/r1",
		RespondString(http.StatusInternalServerError, "application/json", `{"message":"boom"}`))

	r := resourceKubernetesHelm()
	d := r.TestResourceData()
	d.SetId("1:default:r1")

	if err := rcDelete(r, d, mock.Client()); err == nil {
		t.Fatal("expected error on non-204 delete, got nil")
	}
}
