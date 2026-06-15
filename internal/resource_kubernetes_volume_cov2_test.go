package internal

import (
	"net/http"
	"testing"
)

// TestKubernetesVolumesCov2_Update_DeleteThenCreate verifies the Update path,
// which deletes the existing volume (named URL) and then re-creates it from the
// manifest, resulting in a refreshed composite ID.
func TestKubernetesVolumesCov2_Update_DeleteThenCreate(t *testing.T) {
	mock := NewMockServer(t)

	// Delete of the prior named PVC.
	mock.On("DELETE", "/endpoints/1/kubernetes/api/v1/namespaces/default/persistentvolumeclaims/my-pvc",
		RespondString(http.StatusOK, "", ""))
	// Re-create.
	mock.On("POST", "/endpoints/1/kubernetes/api/v1/namespaces/default/persistentvolumeclaims",
		RespondJSON(http.StatusCreated, map[string]interface{}{"kind": "PersistentVolumeClaim"}))

	r := resourceKubernetesVolumes()
	d := r.TestResourceData()
	d.SetId("1:default:persistent-volume-claim:my-pvc")
	_ = d.Set("endpoint_id", 1)
	_ = d.Set("namespace", "default")
	_ = d.Set("type", "persistent-volume-claim")
	_ = d.Set("manifest", k8sPVCManifest)

	if err := rcUpdate(r, d, mock.Client()); err != nil {
		t.Fatalf("Update failed: %v", err)
	}
	if mock.FindRequest("DELETE", "/endpoints/1/kubernetes/api/v1/namespaces/default/persistentvolumeclaims/my-pvc") == nil {
		t.Error("expected DELETE during update")
	}
	if mock.FindRequest("POST", "/endpoints/1/kubernetes/api/v1/namespaces/default/persistentvolumeclaims") == nil {
		t.Error("expected POST during update")
	}
	if d.Id() != "1:default:persistent-volume-claim:my-pvc" {
		t.Errorf("unexpected ID after update %q", d.Id())
	}
}

// TestKubernetesVolumesCov2_Update_DeleteErrorAborts verifies the Update path
// short-circuits when the delete fails (no create attempted).
func TestKubernetesVolumesCov2_Update_DeleteErrorAborts(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("DELETE", "/endpoints/1/kubernetes/api/v1/namespaces/default/persistentvolumeclaims/my-pvc",
		RespondString(http.StatusInternalServerError, "application/json", `{"message":"boom"}`))

	r := resourceKubernetesVolumes()
	d := r.TestResourceData()
	d.SetId("1:default:persistent-volume-claim:my-pvc")
	_ = d.Set("endpoint_id", 1)
	_ = d.Set("namespace", "default")
	_ = d.Set("type", "persistent-volume-claim")
	_ = d.Set("manifest", k8sPVCManifest)

	if err := rcUpdate(r, d, mock.Client()); err == nil {
		t.Fatal("expected Update to fail when delete errors, got nil")
	}
}

// TestKubernetesVolumesCov2_Create_PersistentVolume covers the cluster-scoped
// persistent-volume create branch (URL without namespace segment).
func TestKubernetesVolumesCov2_Create_PersistentVolume(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("POST", "/endpoints/2/kubernetes/api/v1/persistentvolumes",
		RespondJSON(http.StatusCreated, map[string]interface{}{"kind": "PersistentVolume"}))

	r := resourceKubernetesVolumes()
	d := r.TestResourceData()
	_ = d.Set("endpoint_id", 2)
	_ = d.Set("type", "persistent-volume")
	_ = d.Set("manifest", `{"apiVersion":"v1","kind":"PersistentVolume","metadata":{"name":"data-pv"}}`)

	if err := rcCreate(r, d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if d.Id() != "2::persistent-volume:data-pv" {
		t.Errorf("unexpected ID %q", d.Id())
	}
}

// TestKubernetesVolumesCov2_Create_VolumeAttachment covers the
// volume-attachment create branch (storage.k8s.io apis path).
func TestKubernetesVolumesCov2_Create_VolumeAttachment(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("POST", "/endpoints/3/kubernetes/apis/storage.k8s.io/v1/volumeattachments",
		RespondJSON(http.StatusCreated, map[string]interface{}{"kind": "VolumeAttachment"}))

	r := resourceKubernetesVolumes()
	d := r.TestResourceData()
	_ = d.Set("endpoint_id", 3)
	_ = d.Set("type", "volume-attachment")
	_ = d.Set("manifest", `{"apiVersion":"storage.k8s.io/v1","kind":"VolumeAttachment","metadata":{"name":"my-va"}}`)

	if err := rcCreate(r, d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if d.Id() != "3::volume-attachment:my-va" {
		t.Errorf("unexpected ID %q", d.Id())
	}
}
