package internal

import (
	"context"
	"net/http"
	"testing"
)

// TestK8sVolumeCov3_Import_HappyPath verifies the passthrough importer returns a
// single state with the composite ID preserved.
func TestK8sVolumeCov3_Import_HappyPath(t *testing.T) {
	r := resourceKubernetesVolumes()
	d := r.TestResourceData()
	d.SetId("1:default:persistent-volume-claim:my-pvc")

	results, err := r.Importer.StateContext(context.Background(), d, nil)
	if err != nil {
		t.Fatalf("Import failed: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 imported state, got %d", len(results))
	}
	if results[0].Id() != "1:default:persistent-volume-claim:my-pvc" {
		t.Errorf("expected ID preserved, got %q", results[0].Id())
	}
}

// TestK8sVolumeCov3_Read_InvalidID verifies Read errors when the composite ID
// does not parse into the expected four segments.
func TestK8sVolumeCov3_Read_InvalidID(t *testing.T) {
	mock := NewMockServer(t)

	r := resourceKubernetesVolumes()
	d := r.TestResourceData()
	d.SetId("not-a-valid-id")

	if err := rcRead(r, d, mock.Client()); err == nil {
		t.Fatal("expected error for malformed ID, got nil")
	}
}

// TestK8sVolumeCov3_Read_HTTPError verifies a non-404 error from the existence
// GET surfaces as an error.
func TestK8sVolumeCov3_Read_HTTPError(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/endpoints/1/kubernetes/api/v1/namespaces/default/persistentvolumeclaims/my-pvc",
		RespondString(http.StatusInternalServerError, "application/json", `{"message":"boom"}`))

	r := resourceKubernetesVolumes()
	d := r.TestResourceData()
	d.SetId("1:default:persistent-volume-claim:my-pvc")

	if err := rcRead(r, d, mock.Client()); err == nil {
		t.Fatal("expected error on HTTP 500, got nil")
	}
}

// TestK8sVolumeCov3_Create_UnsupportedType covers the volumeAPIURL default
// branch: a valid manifest but an unsupported volume type errors before any
// HTTP call.
func TestK8sVolumeCov3_Create_UnsupportedType(t *testing.T) {
	mock := NewMockServer(t)

	r := resourceKubernetesVolumes()
	d := r.TestResourceData()
	_ = d.Set("endpoint_id", 1)
	_ = d.Set("namespace", "default")
	_ = d.Set("type", "bogus-type")
	_ = d.Set("manifest", `{"apiVersion":"v1","kind":"Foo","metadata":{"name":"x"}}`)

	if err := rcCreate(r, d, mock.Client()); err == nil {
		t.Fatal("expected error for unsupported volume type, got nil")
	}
	if d.Id() != "" {
		t.Errorf("expected empty ID after error, got %q", d.Id())
	}
}
