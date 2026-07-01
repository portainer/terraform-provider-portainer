package internal

import (
	"net/http"
	"testing"
)

// TestKubernetesStorageUpdate_HappyPath exercises Update, which deletes then
// recreates the storage class.
func TestKubernetesStorageUpdate_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("DELETE", "/endpoints/3/kubernetes/apis/storage.k8s.io/v1/storageclasses/fast",
		RespondString(http.StatusOK, "", ""))
	mock.On("POST", "/endpoints/3/kubernetes/apis/storage.k8s.io/v1/storageclasses",
		RespondString(http.StatusCreated, "application/json", `{}`))

	r := resourceKubernetesStorage()
	d := r.TestResourceData()
	d.SetId("3:fast")
	_ = d.Set("endpoint_id", 3)
	_ = d.Set("manifest", `{"metadata":{"name":"fast"}}`)

	if err := rcUpdate(r, d, mock.Client()); err != nil {
		t.Fatalf("Update failed: %v", err)
	}
	if mock.FindRequest("DELETE", "/endpoints/3/kubernetes/apis/storage.k8s.io/v1/storageclasses/fast") == nil {
		t.Error("expected DELETE during update")
	}
	if mock.FindRequest("POST", "/endpoints/3/kubernetes/apis/storage.k8s.io/v1/storageclasses") == nil {
		t.Error("expected POST during update")
	}
	if d.Id() != "3:fast" {
		t.Errorf("expected ID 3:fast, got %q", d.Id())
	}
}

// TestKubernetesStorageDelete_HTTPError verifies a non-2xx (and non-404) delete
// surfaces an error.
func TestKubernetesStorageDelete_HTTPError(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("DELETE", "/endpoints/1/kubernetes/apis/storage.k8s.io/v1/storageclasses/x",
		RespondString(http.StatusInternalServerError, "application/json", `{"message":"boom"}`))

	r := resourceKubernetesStorage()
	d := r.TestResourceData()
	d.SetId("1:x")

	if err := rcDelete(r, d, mock.Client()); err == nil {
		t.Fatal("expected error on HTTP 500 delete, got nil")
	}
}

// TestKubernetesStorageDelete_404IsSuccess verifies a 404 on delete clears the ID.
func TestKubernetesStorageDelete_404IsSuccess(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("DELETE", "/endpoints/1/kubernetes/apis/storage.k8s.io/v1/storageclasses/gone",
		RespondString(http.StatusNotFound, "application/json", `{"message":"not found"}`))

	r := resourceKubernetesStorage()
	d := r.TestResourceData()
	d.SetId("1:gone")

	if err := rcDelete(r, d, mock.Client()); err != nil {
		t.Fatalf("expected 404 delete to succeed, got %v", err)
	}
	if d.Id() != "" {
		t.Errorf("expected cleared ID, got %q", d.Id())
	}
}

// TestKubernetesStorageRead_NoOp verifies Read is a no-op returning nil.
func TestKubernetesStorageRead_NoOp(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("GET", "/endpoints/1/kubernetes/apis/storage.k8s.io/v1/storageclasses/keep",
		RespondString(http.StatusOK, "application/json", "{}"))

	r := resourceKubernetesStorage()
	d := r.TestResourceData()
	d.SetId("1:keep")
	if err := rcRead(r, d, mock.Client()); err != nil {
		t.Fatalf("Read should be no-op, got %v", err)
	}
	if d.Id() != "1:keep" {
		t.Errorf("Read should not change ID, got %q", d.Id())
	}
}

// TestKubernetesStorageCreate_InvalidManifest verifies an unparseable manifest errors.
func TestKubernetesStorageCreate_InvalidManifest(t *testing.T) {
	r := resourceKubernetesStorage()
	d := r.TestResourceData()
	_ = d.Set("endpoint_id", 1)
	_ = d.Set("manifest", "[unterminated")

	if err := rcCreate(r, d, &APIClient{}); err == nil {
		t.Fatal("expected error for invalid manifest")
	}
}

// TestKubernetesStorageCreate_MissingMetadata verifies a manifest without metadata errors.
func TestKubernetesStorageCreate_MissingMetadata(t *testing.T) {
	r := resourceKubernetesStorage()
	d := r.TestResourceData()
	_ = d.Set("endpoint_id", 1)
	_ = d.Set("manifest", `{"kind":"StorageClass"}`)

	if err := rcCreate(r, d, &APIClient{}); err == nil {
		t.Fatal("expected error for missing metadata")
	}
}

// TestKubernetesStorageCreate_MissingMetadataName verifies a missing name errors.
func TestKubernetesStorageCreate_MissingMetadataName(t *testing.T) {
	r := resourceKubernetesStorage()
	d := r.TestResourceData()
	_ = d.Set("endpoint_id", 1)
	_ = d.Set("manifest", `{"metadata":{}}`)

	if err := rcCreate(r, d, &APIClient{}); err == nil {
		t.Fatal("expected error for missing metadata.name")
	}
}

// TestKubernetesStorageParseID_Malformed covers the <2 parts branch.
func TestKubernetesStorageParseID_Malformed(t *testing.T) {
	endpointID, name := parseStorageID("bad")
	if endpointID != 0 || name != "" {
		t.Errorf("expected zero values, got (%d, %q)", endpointID, name)
	}
}

func TestKubernetesStorageRead_404ClearsID(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("GET", "/endpoints/1/kubernetes/apis/storage.k8s.io/v1/storageclasses/gone",
		RespondString(http.StatusNotFound, "application/json", "{\"message\":\"not found\"}"))

	r := resourceKubernetesStorage()
	d := r.TestResourceData()
	d.SetId("1:gone")

	if err := rcRead(r, d, mock.Client()); err != nil {
		t.Fatalf("Read on 404 should not error, got %v", err)
	}
	if d.Id() != "" {
		t.Errorf("expected ID cleared on 404, got %q", d.Id())
	}
}
