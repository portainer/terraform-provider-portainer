package internal

import (
	"context"
	"net/http"
	"testing"
)

// TestK8sApplicationCov3_Import_HappyPath verifies the passthrough importer
// returns a single state with the composite ID preserved.
func TestK8sApplicationCov3_Import_HappyPath(t *testing.T) {
	r := resourceKubernetesApplication()
	d := r.TestResourceData()
	d.SetId("1:default:my-app")

	results, err := r.Importer.StateContext(context.Background(), d, nil)
	if err != nil {
		t.Fatalf("Import failed: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 imported state, got %d", len(results))
	}
	if results[0].Id() != "1:default:my-app" {
		t.Errorf("expected ID preserved, got %q", results[0].Id())
	}
}

// TestK8sApplicationCov3_Read_InvalidID verifies Read errors when the composite
// ID cannot be parsed into endpoint/namespace/name.
func TestK8sApplicationCov3_Read_InvalidID(t *testing.T) {
	mock := NewMockServer(t)

	r := resourceKubernetesApplication()
	d := r.TestResourceData()
	d.SetId("only-one-part")

	if err := rcRead(r, d, mock.Client()); err == nil {
		t.Fatal("expected error for malformed ID, got nil")
	}
}

// TestK8sApplicationCov3_Read_HappyPath verifies Read confirms existence via GET
// and repopulates the endpoint_id and namespace identity fields.
func TestK8sApplicationCov3_Read_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/endpoints/1/kubernetes/apis/apps/v1/namespaces/default/deployments/my-app",
		RespondJSON(http.StatusOK, map[string]interface{}{"kind": "Deployment"}))

	r := resourceKubernetesApplication()
	d := r.TestResourceData()
	d.SetId("1:default:my-app")

	if err := rcRead(r, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if got := d.Get("endpoint_id"); got != 1 {
		t.Errorf("endpoint_id: expected 1, got %v", got)
	}
	if got := d.Get("namespace"); got != "default" {
		t.Errorf("namespace: expected %q, got %v", "default", got)
	}
	if mock.FindRequest("GET", "/endpoints/1/kubernetes/apis/apps/v1/namespaces/default/deployments/my-app") == nil {
		t.Error("expected existence GET to be recorded")
	}
}

// TestK8sApplicationCov3_Read_HTTPError verifies a non-404 error from the
// existence GET surfaces as an error.
func TestK8sApplicationCov3_Read_HTTPError(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/endpoints/1/kubernetes/apis/apps/v1/namespaces/default/deployments/my-app",
		RespondString(http.StatusInternalServerError, "application/json", `{"message":"boom"}`))

	r := resourceKubernetesApplication()
	d := r.TestResourceData()
	d.SetId("1:default:my-app")

	if err := rcRead(r, d, mock.Client()); err == nil {
		t.Fatal("expected error on HTTP 500, got nil")
	}
}
