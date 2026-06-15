package internal

import (
	"net/http"
	"testing"
)

// TestKubernetesRoleBindingCreate_InvalidManifest covers the parse error branch.
func TestKubernetesRoleBindingCreate_InvalidManifest(t *testing.T) {
	mock := NewMockServer(t)

	r := resourceKubernetesRoleBindings()
	d := r.TestResourceData()
	_ = d.Set("endpoint_id", 1)
	_ = d.Set("namespace", "default")
	_ = d.Set("manifest", "::: not valid :::")

	if err := rcCreate(r, d, mock.Client()); err == nil {
		t.Fatal("expected error for invalid manifest, got nil")
	}
}

// TestKubernetesRoleBindingCreate_MissingMetadata covers the missing-metadata branch.
func TestKubernetesRoleBindingCreate_MissingMetadata(t *testing.T) {
	mock := NewMockServer(t)

	r := resourceKubernetesRoleBindings()
	d := r.TestResourceData()
	_ = d.Set("endpoint_id", 1)
	_ = d.Set("namespace", "default")
	_ = d.Set("manifest", `{"kind":"RoleBinding"}`)

	if err := rcCreate(r, d, mock.Client()); err == nil {
		t.Fatal("expected error for missing metadata, got nil")
	}
}

// TestKubernetesRoleBindingCreate_MissingName covers the missing metadata.name branch.
func TestKubernetesRoleBindingCreate_MissingName(t *testing.T) {
	mock := NewMockServer(t)

	r := resourceKubernetesRoleBindings()
	d := r.TestResourceData()
	_ = d.Set("endpoint_id", 1)
	_ = d.Set("namespace", "default")
	_ = d.Set("manifest", `{"kind":"RoleBinding","metadata":{}}`)

	if err := rcCreate(r, d, mock.Client()); err == nil {
		t.Fatal("expected error for missing metadata.name, got nil")
	}
}

// TestKubernetesRoleBindingReadNoop covers the no-op Read handler.
func TestKubernetesRoleBindingReadNoop(t *testing.T) {
	mock := NewMockServer(t)

	r := resourceKubernetesRoleBindings()
	d := r.TestResourceData()
	d.SetId("1:default:read-pods")

	if err := rcRead(r, d, mock.Client()); err != nil {
		t.Fatalf("Read noop should not error: %v", err)
	}
}

// TestKubernetesRoleBindingDelete_404IsSuccess covers the 404-tolerant delete branch.
func TestKubernetesRoleBindingDelete_404IsSuccess(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("DELETE", "/endpoints/1/kubernetes/apis/rbac.authorization.k8s.io/v1/namespaces/default/rolebindings/read-pods",
		RespondString(http.StatusNotFound, "application/json", `{"message":"gone"}`))

	r := resourceKubernetesRoleBindings()
	d := r.TestResourceData()
	d.SetId("1:default:read-pods")

	if err := rcDelete(r, d, mock.Client()); err != nil {
		t.Fatalf("Delete should treat 404 as success, got: %v", err)
	}
	if d.Id() != "" {
		t.Errorf("expected ID cleared, got %q", d.Id())
	}
}

// TestKubernetesRoleBindingDelete_HTTPError covers the delete error branch.
func TestKubernetesRoleBindingDelete_HTTPError(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("DELETE", "/endpoints/1/kubernetes/apis/rbac.authorization.k8s.io/v1/namespaces/default/rolebindings/read-pods",
		RespondString(http.StatusInternalServerError, "application/json", `{"message":"boom"}`))

	r := resourceKubernetesRoleBindings()
	d := r.TestResourceData()
	d.SetId("1:default:read-pods")

	if err := rcDelete(r, d, mock.Client()); err == nil {
		t.Fatal("expected error on DELETE 500, got nil")
	}
}

// TestKubernetesRoleBindingUpdate_HappyPath covers Update (delete + create).
func TestKubernetesRoleBindingUpdate_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("DELETE", "/endpoints/1/kubernetes/apis/rbac.authorization.k8s.io/v1/namespaces/default/rolebindings/read-pods",
		RespondString(http.StatusOK, "", ""))
	mock.On("POST", "/endpoints/1/kubernetes/apis/rbac.authorization.k8s.io/v1/namespaces/default/rolebindings",
		RespondJSON(http.StatusCreated, map[string]interface{}{"kind": "RoleBinding"}))

	r := resourceKubernetesRoleBindings()
	d := r.TestResourceData()
	d.SetId("1:default:read-pods")
	_ = d.Set("endpoint_id", 1)
	_ = d.Set("namespace", "default")
	_ = d.Set("manifest", roleBindingManifestJSON)

	if err := rcUpdate(r, d, mock.Client()); err != nil {
		t.Fatalf("Update failed: %v", err)
	}
	if d.Id() != "1:default:read-pods" {
		t.Errorf("expected ID re-set after update, got %q", d.Id())
	}
}

// TestKubernetesRoleBindingParseID_Malformed covers the malformed-ID branch.
func TestKubernetesRoleBindingParseID_Malformed(t *testing.T) {
	endpointID, namespace, name := parseRoleBindingsID("1:onlytwo")
	if endpointID != 0 || namespace != "" || name != "" {
		t.Errorf("expected zero values on malformed ID, got (%d, %q, %q)", endpointID, namespace, name)
	}
}
