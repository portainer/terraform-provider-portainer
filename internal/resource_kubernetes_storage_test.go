package internal

import (
	"net/http"
	"testing"
)

const storageClassManifestJSON = `{
  "apiVersion": "storage.k8s.io/v1",
  "kind": "StorageClass",
  "metadata": {"name": "fast-ssd"},
  "provisioner": "kubernetes.io/gce-pd"
}`

// TestKubernetesStorageCreate_HappyPath verifies POST and ID.
func TestKubernetesStorageCreate_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("POST", "/endpoints/1/kubernetes/apis/storage.k8s.io/v1/storageclasses",
		RespondJSON(http.StatusCreated, map[string]interface{}{"kind": "StorageClass"}))

	r := resourceKubernetesStorage()
	d := r.TestResourceData()
	_ = d.Set("endpoint_id", 1)
	_ = d.Set("manifest", storageClassManifestJSON)

	if err := rcCreate(r, d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if d.Id() != "1:fast-ssd" {
		t.Errorf("expected ID %q, got %q", "1:fast-ssd", d.Id())
	}

	post := mock.FindRequest("POST", "/endpoints/1/kubernetes/apis/storage.k8s.io/v1/storageclasses")
	if post == nil {
		t.Fatal("expected POST request to be recorded")
	}
	var payload map[string]interface{}
	if err := post.DecodeJSON(&payload); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if payload["kind"] != "StorageClass" {
		t.Errorf("payload.kind: expected %q, got %v", "StorageClass", payload["kind"])
	}
	if payload["provisioner"] != "kubernetes.io/gce-pd" {
		t.Errorf("payload.provisioner: expected %q, got %v", "kubernetes.io/gce-pd", payload["provisioner"])
	}
}

// TestKubernetesStorageCreate_HTTPError verifies HTTP error surfaces.
func TestKubernetesStorageCreate_HTTPError(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("POST", "/endpoints/1/kubernetes/apis/storage.k8s.io/v1/storageclasses",
		RespondString(http.StatusInternalServerError, "application/json", `{"message":"boom"}`))

	r := resourceKubernetesStorage()
	d := r.TestResourceData()
	_ = d.Set("endpoint_id", 1)
	_ = d.Set("manifest", storageClassManifestJSON)

	if err := rcCreate(r, d, mock.Client()); err == nil {
		t.Fatal("expected error on HTTP 500, got nil")
	}
}

// TestKubernetesStorageDelete_HappyPath verifies DELETE.
func TestKubernetesStorageDelete_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("DELETE", "/endpoints/1/kubernetes/apis/storage.k8s.io/v1/storageclasses/fast-ssd",
		RespondString(http.StatusOK, "", ""))

	r := resourceKubernetesStorage()
	d := r.TestResourceData()
	d.SetId("1:fast-ssd")

	if err := rcDelete(r, d, mock.Client()); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	if mock.FindRequest("DELETE", "/endpoints/1/kubernetes/apis/storage.k8s.io/v1/storageclasses/fast-ssd") == nil {
		t.Error("expected DELETE request to be recorded")
	}
}

// TestKubernetesStorageParseID verifies ID parsing.
func TestKubernetesStorageParseID(t *testing.T) {
	endpointID, name := parseStorageID("8:hot")
	if endpointID != 8 || name != "hot" {
		t.Errorf("expected (8, hot), got (%d, %q)", endpointID, name)
	}
}
