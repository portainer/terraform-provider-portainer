package internal

import (
	"net/http"
	"testing"
)

// TestKubernetesRoleCreate_InvalidManifest covers the parse error branch.
func TestKubernetesRoleCreate_InvalidManifest(t *testing.T) {
	mock := NewMockServer(t)

	r := resourceKubernetesRoles()
	d := r.TestResourceData()
	_ = d.Set("endpoint_id", 1)
	_ = d.Set("namespace", "default")
	_ = d.Set("manifest", "::: not valid :::")

	if err := rcCreate(r, d, mock.Client()); err == nil {
		t.Fatal("expected error for invalid manifest, got nil")
	}
}

// TestKubernetesRoleCreate_MissingMetadata covers the missing-metadata branch.
func TestKubernetesRoleCreate_MissingMetadata(t *testing.T) {
	mock := NewMockServer(t)

	r := resourceKubernetesRoles()
	d := r.TestResourceData()
	_ = d.Set("endpoint_id", 1)
	_ = d.Set("namespace", "default")
	_ = d.Set("manifest", `{"kind":"Role"}`)

	if err := rcCreate(r, d, mock.Client()); err == nil {
		t.Fatal("expected error for missing metadata, got nil")
	}
}

// TestKubernetesRoleCreate_MissingName covers the missing metadata.name branch.
func TestKubernetesRoleCreate_MissingName(t *testing.T) {
	mock := NewMockServer(t)

	r := resourceKubernetesRoles()
	d := r.TestResourceData()
	_ = d.Set("endpoint_id", 1)
	_ = d.Set("namespace", "default")
	_ = d.Set("manifest", `{"kind":"Role","metadata":{}}`)

	if err := rcCreate(r, d, mock.Client()); err == nil {
		t.Fatal("expected error for missing metadata.name, got nil")
	}
}

// TestKubernetesRoleReadNoop covers the no-op Read handler.
func TestKubernetesRoleReadNoop(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("GET", "/endpoints/1/kubernetes/apis/rbac.authorization.k8s.io/v1/namespaces/default/roles/pod-reader",
		RespondString(http.StatusOK, "application/json", "{}"))

	r := resourceKubernetesRoles()
	d := r.TestResourceData()
	d.SetId("1:default:pod-reader")

	if err := rcRead(r, d, mock.Client()); err != nil {
		t.Fatalf("Read noop should not error: %v", err)
	}
}

// TestKubernetesRoleDelete_HTTPError covers the delete error branch.
func TestKubernetesRoleDelete_HTTPError(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("DELETE", "/endpoints/1/kubernetes/apis/rbac.authorization.k8s.io/v1/namespaces/default/roles/pod-reader",
		RespondString(http.StatusInternalServerError, "application/json", `{"message":"boom"}`))

	r := resourceKubernetesRoles()
	d := r.TestResourceData()
	d.SetId("1:default:pod-reader")

	if err := rcDelete(r, d, mock.Client()); err == nil {
		t.Fatal("expected error on DELETE 500, got nil")
	}
}

// TestKubernetesRoleUpdate_HappyPath covers Update (delete + create).
func TestKubernetesRoleUpdate_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("DELETE", "/endpoints/1/kubernetes/apis/rbac.authorization.k8s.io/v1/namespaces/default/roles/pod-reader",
		RespondString(http.StatusOK, "", ""))
	mock.On("POST", "/endpoints/1/kubernetes/apis/rbac.authorization.k8s.io/v1/namespaces/default/roles",
		RespondJSON(http.StatusCreated, map[string]interface{}{"kind": "Role"}))

	r := resourceKubernetesRoles()
	d := r.TestResourceData()
	d.SetId("1:default:pod-reader")
	_ = d.Set("endpoint_id", 1)
	_ = d.Set("namespace", "default")
	_ = d.Set("manifest", roleManifestJSON)

	if err := rcUpdate(r, d, mock.Client()); err != nil {
		t.Fatalf("Update failed: %v", err)
	}
	if d.Id() != "1:default:pod-reader" {
		t.Errorf("expected ID re-set after update, got %q", d.Id())
	}
}

// TestKubernetesRoleParseID_Malformed covers the malformed-ID branch.
func TestKubernetesRoleParseID_Malformed(t *testing.T) {
	endpointID, namespace, name := parseRolesID("1:onlytwo")
	if endpointID != 0 || namespace != "" || name != "" {
		t.Errorf("expected zero values on malformed ID, got (%d, %q, %q)", endpointID, namespace, name)
	}
}
