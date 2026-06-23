package internal

import (
	"net/http"
	"testing"
)

// TestKubernetesClusterRoleBindingCreate_MissingMetadata covers the missing-metadata branch.
func TestKubernetesClusterRoleBindingCreate_MissingMetadata(t *testing.T) {
	mock := NewMockServer(t)

	r := resourceKubernetesClusterRoleBindings()
	d := r.TestResourceData()
	_ = d.Set("endpoint_id", 1)
	_ = d.Set("manifest", `{"kind":"ClusterRoleBinding"}`)

	if err := rcCreate(r, d, mock.Client()); err == nil {
		t.Fatal("expected error for missing metadata, got nil")
	}
}

// TestKubernetesClusterRoleBindingCreate_MissingName covers the missing metadata.name branch.
func TestKubernetesClusterRoleBindingCreate_MissingName(t *testing.T) {
	mock := NewMockServer(t)

	r := resourceKubernetesClusterRoleBindings()
	d := r.TestResourceData()
	_ = d.Set("endpoint_id", 1)
	_ = d.Set("manifest", `{"kind":"ClusterRoleBinding","metadata":{}}`)

	if err := rcCreate(r, d, mock.Client()); err == nil {
		t.Fatal("expected error for missing metadata.name, got nil")
	}
}

// TestKubernetesClusterRoleBindingCreate_HTTPError covers the create error branch.
func TestKubernetesClusterRoleBindingCreate_HTTPError(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("POST", "/endpoints/1/kubernetes/apis/rbac.authorization.k8s.io/v1/clusterrolebindings",
		RespondString(http.StatusForbidden, "application/json", `{"message":"nope"}`))

	r := resourceKubernetesClusterRoleBindings()
	d := r.TestResourceData()
	_ = d.Set("endpoint_id", 1)
	_ = d.Set("manifest", clusterRoleBindingManifestJSON)

	if err := rcCreate(r, d, mock.Client()); err == nil {
		t.Fatal("expected error on HTTP 403, got nil")
	}
}

// TestKubernetesClusterRoleBindingReadNoop covers the no-op Read handler.
func TestKubernetesClusterRoleBindingReadNoop(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("GET", "/endpoints/1/kubernetes/apis/rbac.authorization.k8s.io/v1/clusterrolebindings/global-admin",
		RespondString(http.StatusOK, "application/json", "{}"))

	r := resourceKubernetesClusterRoleBindings()
	d := r.TestResourceData()
	d.SetId("1:global-admin")

	if err := rcRead(r, d, mock.Client()); err != nil {
		t.Fatalf("Read noop should not error: %v", err)
	}
}

// TestKubernetesClusterRoleBindingDelete_HTTPError covers the delete error branch.
func TestKubernetesClusterRoleBindingDelete_HTTPError(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("DELETE", "/endpoints/1/kubernetes/apis/rbac.authorization.k8s.io/v1/clusterrolebindings/global-admin",
		RespondString(http.StatusInternalServerError, "application/json", `{"message":"boom"}`))

	r := resourceKubernetesClusterRoleBindings()
	d := r.TestResourceData()
	d.SetId("1:global-admin")

	if err := rcDelete(r, d, mock.Client()); err == nil {
		t.Fatal("expected error on DELETE 500, got nil")
	}
}

// TestKubernetesClusterRoleBindingUpdate_HappyPath covers Update (delete + create).
func TestKubernetesClusterRoleBindingUpdate_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("DELETE", "/endpoints/1/kubernetes/apis/rbac.authorization.k8s.io/v1/clusterrolebindings/global-admin",
		RespondString(http.StatusOK, "", ""))
	mock.On("POST", "/endpoints/1/kubernetes/apis/rbac.authorization.k8s.io/v1/clusterrolebindings",
		RespondJSON(http.StatusCreated, map[string]interface{}{"kind": "ClusterRoleBinding"}))

	r := resourceKubernetesClusterRoleBindings()
	d := r.TestResourceData()
	d.SetId("1:global-admin")
	_ = d.Set("endpoint_id", 1)
	_ = d.Set("manifest", clusterRoleBindingManifestJSON)

	if err := rcUpdate(r, d, mock.Client()); err != nil {
		t.Fatalf("Update failed: %v", err)
	}
	if d.Id() != "1:global-admin" {
		t.Errorf("expected ID re-set after update, got %q", d.Id())
	}
}

// TestKubernetesClusterRoleBindingParseID_Malformed covers the malformed-ID branch.
func TestKubernetesClusterRoleBindingParseID_Malformed(t *testing.T) {
	endpointID, name := parseClusterRolesBindingsID("noseparator")
	if endpointID != 0 || name != "" {
		t.Errorf("expected zero values on malformed ID, got (%d, %q)", endpointID, name)
	}
}
