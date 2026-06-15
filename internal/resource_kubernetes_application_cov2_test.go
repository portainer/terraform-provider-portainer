package internal

import (
	"net/http"
	"testing"
)

// TestKubernetesApplicationCov2_ParseID covers the composite-ID parser,
// including the malformed-input branch.
func TestKubernetesApplicationCov2_ParseID(t *testing.T) {
	tests := []struct {
		name      string
		id        string
		wantEndpt int
		wantNS    string
		wantName  string
	}{
		{"valid", "1:default:my-app", 1, "default", "my-app"},
		{"name has colon", "2:ns:a:b", 2, "ns", "a:b"},
		{"too few parts", "1:default", 0, "", ""},
		{"empty", "", 0, "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			endpt, ns, name := parseApllicationsID(tt.id)
			if endpt != tt.wantEndpt || ns != tt.wantNS || name != tt.wantName {
				t.Errorf("parseApllicationsID(%q) = (%d,%q,%q), want (%d,%q,%q)",
					tt.id, endpt, ns, name, tt.wantEndpt, tt.wantNS, tt.wantName)
			}
		})
	}
}

// TestKubernetesApplicationCov2_Create_BadManifest verifies an unparseable
// manifest surfaces an error before any HTTP call.
func TestKubernetesApplicationCov2_Create_BadManifest(t *testing.T) {
	mock := NewMockServer(t)

	r := resourceKubernetesApplication()
	d := r.TestResourceData()
	_ = d.Set("endpoint_id", 1)
	_ = d.Set("namespace", "default")
	_ = d.Set("manifest", ":::not valid yaml or json:::\n\t- [")

	if err := rcCreate(r, d, mock.Client()); err == nil {
		t.Fatal("expected error for invalid manifest, got nil")
	}
}

// TestKubernetesApplicationCov2_Create_MissingMetadata verifies a manifest
// without a metadata block surfaces an error.
func TestKubernetesApplicationCov2_Create_MissingMetadata(t *testing.T) {
	mock := NewMockServer(t)

	r := resourceKubernetesApplication()
	d := r.TestResourceData()
	_ = d.Set("endpoint_id", 1)
	_ = d.Set("namespace", "default")
	_ = d.Set("manifest", `{"kind":"Deployment"}`)

	if err := rcCreate(r, d, mock.Client()); err == nil {
		t.Fatal("expected error for missing metadata, got nil")
	}
}

// TestKubernetesApplicationCov2_Update_DeleteThenCreate verifies the Update
// path deletes the existing deployment then re-creates it.
func TestKubernetesApplicationCov2_Update_DeleteThenCreate(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("DELETE", "/endpoints/1/kubernetes/apis/apps/v1/namespaces/default/deployments/my-app",
		RespondJSON(http.StatusOK, map[string]interface{}{}))
	mock.On("POST", "/endpoints/1/kubernetes/apis/apps/v1/namespaces/default/deployments",
		RespondJSON(http.StatusOK, map[string]interface{}{}))

	r := resourceKubernetesApplication()
	d := r.TestResourceData()
	d.SetId("1:default:my-app")
	_ = d.Set("endpoint_id", 1)
	_ = d.Set("namespace", "default")
	_ = d.Set("manifest", `{"kind":"Deployment","metadata":{"name":"my-app"}}`)

	if err := rcUpdate(r, d, mock.Client()); err != nil {
		t.Fatalf("Update failed: %v", err)
	}
	if mock.FindRequest("DELETE", "/endpoints/1/kubernetes/apis/apps/v1/namespaces/default/deployments/my-app") == nil {
		t.Error("expected DELETE during update")
	}
	if mock.FindRequest("POST", "/endpoints/1/kubernetes/apis/apps/v1/namespaces/default/deployments") == nil {
		t.Error("expected POST during update")
	}
	if d.Id() != "1:default:my-app" {
		t.Errorf("unexpected ID after update %q", d.Id())
	}
}

// TestKubernetesApplicationCov2_Update_DeleteErrorAborts verifies the Update
// path short-circuits if the delete returns a hard error.
func TestKubernetesApplicationCov2_Update_DeleteErrorAborts(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("DELETE", "/endpoints/1/kubernetes/apis/apps/v1/namespaces/default/deployments/my-app",
		RespondString(http.StatusInternalServerError, "application/json", `{"message":"boom"}`))

	r := resourceKubernetesApplication()
	d := r.TestResourceData()
	d.SetId("1:default:my-app")
	_ = d.Set("endpoint_id", 1)
	_ = d.Set("namespace", "default")
	_ = d.Set("manifest", `{"kind":"Deployment","metadata":{"name":"my-app"}}`)

	if err := rcUpdate(r, d, mock.Client()); err == nil {
		t.Fatal("expected Update to fail when delete errors, got nil")
	}
}

// TestKubernetesApplicationCov2_Delete_404IsSuccess verifies a 404 on delete
// is treated as success (already gone).
func TestKubernetesApplicationCov2_Delete_404IsSuccess(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("DELETE", "/endpoints/1/kubernetes/apis/apps/v1/namespaces/default/deployments/my-app",
		RespondString(http.StatusNotFound, "application/json", `{"message":"gone"}`))

	r := resourceKubernetesApplication()
	d := r.TestResourceData()
	d.SetId("1:default:my-app")

	if err := rcDelete(r, d, mock.Client()); err != nil {
		t.Fatalf("Delete should swallow 404, got error: %v", err)
	}
	if d.Id() != "" {
		t.Errorf("expected ID cleared, got %q", d.Id())
	}
}

// TestKubernetesApplicationCov2_Delete_HTTPError verifies a non-2xx/non-404
// delete surfaces an error.
func TestKubernetesApplicationCov2_Delete_HTTPError(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("DELETE", "/endpoints/1/kubernetes/apis/apps/v1/namespaces/default/deployments/my-app",
		RespondString(http.StatusInternalServerError, "application/json", `{"message":"boom"}`))

	r := resourceKubernetesApplication()
	d := r.TestResourceData()
	d.SetId("1:default:my-app")

	if err := rcDelete(r, d, mock.Client()); err == nil {
		t.Fatal("expected error on HTTP 500, got nil")
	}
}
