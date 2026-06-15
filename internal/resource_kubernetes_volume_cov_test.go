package internal

import (
	"net/http"
	"testing"
)

// TestKubernetesVolumesParseVolumesID covers the composite-ID parser including
// the malformed-input branch.
func TestKubernetesVolumesParseVolumesID(t *testing.T) {
	tests := []struct {
		name        string
		id          string
		wantEndpt   int
		wantNS      string
		wantType    string
		wantVolName string
	}{
		{
			name:        "valid PVC id",
			id:          "1:default:persistent-volume-claim:my-pvc",
			wantEndpt:   1,
			wantNS:      "default",
			wantType:    "persistent-volume-claim",
			wantVolName: "my-pvc",
		},
		{
			name:        "name contains colon",
			id:          "2:ns:persistent-volume:a:b",
			wantEndpt:   2,
			wantNS:      "ns",
			wantType:    "persistent-volume",
			wantVolName: "a:b",
		},
		{
			name:        "too few parts returns zero values",
			id:          "1:default",
			wantEndpt:   0,
			wantNS:      "",
			wantType:    "",
			wantVolName: "",
		},
		{
			name:        "empty string",
			id:          "",
			wantEndpt:   0,
			wantNS:      "",
			wantType:    "",
			wantVolName: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			endpt, ns, vt, name := parseVolumesID(tt.id)
			if endpt != tt.wantEndpt {
				t.Errorf("endpoint: expected %d, got %d", tt.wantEndpt, endpt)
			}
			if ns != tt.wantNS {
				t.Errorf("namespace: expected %q, got %q", tt.wantNS, ns)
			}
			if vt != tt.wantType {
				t.Errorf("type: expected %q, got %q", tt.wantType, vt)
			}
			if name != tt.wantVolName {
				t.Errorf("name: expected %q, got %q", tt.wantVolName, name)
			}
		})
	}
}

const k8sPVCManifest = `{"apiVersion":"v1","kind":"PersistentVolumeClaim","metadata":{"name":"my-pvc"}}`

// TestKubernetesVolumesCreate_HappyPath verifies the POST is sent to the PVC
// URL and the composite ID is set.
func TestKubernetesVolumesCreate_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("POST", "/endpoints/1/kubernetes/api/v1/namespaces/default/persistentvolumeclaims",
		RespondJSON(http.StatusCreated, map[string]interface{}{"kind": "PersistentVolumeClaim"}))

	r := resourceKubernetesVolumes()
	d := r.TestResourceData()
	_ = d.Set("endpoint_id", 1)
	_ = d.Set("namespace", "default")
	_ = d.Set("type", "persistent-volume-claim")
	_ = d.Set("manifest", k8sPVCManifest)

	if err := rcCreate(r, d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	if d.Id() != "1:default:persistent-volume-claim:my-pvc" {
		t.Errorf("unexpected ID %q", d.Id())
	}
	if mock.FindRequest("POST", "/endpoints/1/kubernetes/api/v1/namespaces/default/persistentvolumeclaims") == nil {
		t.Error("expected POST to PVC endpoint")
	}
}

// TestKubernetesVolumesCreate_BadManifest verifies an invalid manifest errors.
func TestKubernetesVolumesCreate_BadManifest(t *testing.T) {
	mock := NewMockServer(t)

	r := resourceKubernetesVolumes()
	d := r.TestResourceData()
	_ = d.Set("endpoint_id", 1)
	_ = d.Set("namespace", "default")
	_ = d.Set("type", "persistent-volume-claim")
	_ = d.Set("manifest", ":::not valid yaml or json:::\n\t- [")

	if err := rcCreate(r, d, mock.Client()); err == nil {
		t.Fatal("expected error for invalid manifest, got nil")
	}
}

// TestKubernetesVolumesCreate_MissingMetadataName verifies a manifest without
// metadata.name errors.
func TestKubernetesVolumesCreate_MissingMetadataName(t *testing.T) {
	mock := NewMockServer(t)

	r := resourceKubernetesVolumes()
	d := r.TestResourceData()
	_ = d.Set("endpoint_id", 1)
	_ = d.Set("namespace", "default")
	_ = d.Set("type", "persistent-volume-claim")
	_ = d.Set("manifest", `{"apiVersion":"v1","metadata":{}}`)

	if err := rcCreate(r, d, mock.Client()); err == nil {
		t.Fatal("expected error for missing metadata.name, got nil")
	}
}

// TestKubernetesVolumesCreate_MissingMetadata verifies a manifest without a
// metadata block errors.
func TestKubernetesVolumesCreate_MissingMetadata(t *testing.T) {
	mock := NewMockServer(t)

	r := resourceKubernetesVolumes()
	d := r.TestResourceData()
	_ = d.Set("endpoint_id", 1)
	_ = d.Set("namespace", "default")
	_ = d.Set("type", "persistent-volume-claim")
	_ = d.Set("manifest", `{"apiVersion":"v1","kind":"PersistentVolumeClaim"}`)

	if err := rcCreate(r, d, mock.Client()); err == nil {
		t.Fatal("expected error for missing metadata, got nil")
	}
}

// TestKubernetesVolumesCreate_HTTPError verifies a non-2xx create surfaces an error.
func TestKubernetesVolumesCreate_HTTPError(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("POST", "/endpoints/1/kubernetes/api/v1/namespaces/default/persistentvolumeclaims",
		RespondString(http.StatusInternalServerError, "application/json", `{"message":"boom"}`))

	r := resourceKubernetesVolumes()
	d := r.TestResourceData()
	_ = d.Set("endpoint_id", 1)
	_ = d.Set("namespace", "default")
	_ = d.Set("type", "persistent-volume-claim")
	_ = d.Set("manifest", k8sPVCManifest)

	if err := rcCreate(r, d, mock.Client()); err == nil {
		t.Fatal("expected error on HTTP 500, got nil")
	}
}

// TestKubernetesVolumesRead_NoOp verifies Read is a no-op (returns nil).
func TestKubernetesVolumesRead_NoOp(t *testing.T) {
	mock := NewMockServer(t)

	r := resourceKubernetesVolumes()
	d := r.TestResourceData()
	d.SetId("1:default:persistent-volume-claim:my-pvc")

	if err := rcRead(r, d, mock.Client()); err != nil {
		t.Fatalf("Read should be a no-op, got error: %v", err)
	}
}

// TestKubernetesVolumesDelete_HappyPath verifies DELETE is sent to the named
// PVC URL and the ID is cleared.
func TestKubernetesVolumesDelete_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("DELETE", "/endpoints/1/kubernetes/api/v1/namespaces/default/persistentvolumeclaims/my-pvc",
		RespondString(http.StatusOK, "", ""))

	r := resourceKubernetesVolumes()
	d := r.TestResourceData()
	d.SetId("1:default:persistent-volume-claim:my-pvc")

	if err := rcDelete(r, d, mock.Client()); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	if mock.FindRequest("DELETE", "/endpoints/1/kubernetes/api/v1/namespaces/default/persistentvolumeclaims/my-pvc") == nil {
		t.Error("expected DELETE to named PVC endpoint")
	}
	if d.Id() != "" {
		t.Errorf("expected ID cleared, got %q", d.Id())
	}
}

// TestKubernetesVolumesDelete_404IsSuccess verifies a 404 on delete is success.
func TestKubernetesVolumesDelete_404IsSuccess(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("DELETE", "/endpoints/1/kubernetes/api/v1/persistentvolumes/my-pv",
		RespondString(http.StatusNotFound, "application/json", `{"message":"gone"}`))

	r := resourceKubernetesVolumes()
	d := r.TestResourceData()
	d.SetId("1::persistent-volume:my-pv")

	if err := rcDelete(r, d, mock.Client()); err != nil {
		t.Fatalf("Delete should swallow 404, got error: %v", err)
	}
}

// TestKubernetesVolumesDelete_HTTPError verifies a non-2xx/non-404 delete errors.
func TestKubernetesVolumesDelete_HTTPError(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("DELETE", "/endpoints/1/kubernetes/api/v1/namespaces/default/persistentvolumeclaims/my-pvc",
		RespondString(http.StatusInternalServerError, "application/json", `{"message":"boom"}`))

	r := resourceKubernetesVolumes()
	d := r.TestResourceData()
	d.SetId("1:default:persistent-volume-claim:my-pvc")

	if err := rcDelete(r, d, mock.Client()); err == nil {
		t.Fatal("expected error on HTTP 500, got nil")
	}
}
