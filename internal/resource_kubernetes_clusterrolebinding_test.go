package internal

import (
	"net/http"
	"testing"
)

const clusterRoleBindingManifestJSON = `{
  "apiVersion": "rbac.authorization.k8s.io/v1",
  "kind": "ClusterRoleBinding",
  "metadata": {"name": "global-admin"},
  "subjects": [{"kind": "User", "name": "alice"}],
  "roleRef": {"kind": "ClusterRole", "name": "cluster-admin", "apiGroup": "rbac.authorization.k8s.io"}
}`

// TestKubernetesClusterRoleBindingCreate_HappyPath verifies POST and ID.
func TestKubernetesClusterRoleBindingCreate_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("POST", "/endpoints/1/kubernetes/apis/rbac.authorization.k8s.io/v1/clusterrolebindings",
		RespondJSON(http.StatusCreated, map[string]interface{}{"kind": "ClusterRoleBinding"}))

	r := resourceKubernetesClusterRoleBindings()
	d := r.TestResourceData()
	_ = d.Set("endpoint_id", 1)
	_ = d.Set("manifest", clusterRoleBindingManifestJSON)

	if err := rcCreate(r, d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if d.Id() != "1:global-admin" {
		t.Errorf("expected ID %q, got %q", "1:global-admin", d.Id())
	}

	post := mock.FindRequest("POST", "/endpoints/1/kubernetes/apis/rbac.authorization.k8s.io/v1/clusterrolebindings")
	if post == nil {
		t.Fatal("expected POST request to be recorded")
	}
	var payload map[string]interface{}
	if err := post.DecodeJSON(&payload); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if payload["kind"] != "ClusterRoleBinding" {
		t.Errorf("payload.kind: expected %q, got %v", "ClusterRoleBinding", payload["kind"])
	}
}

// TestKubernetesClusterRoleBindingCreate_InvalidManifest verifies parser error.
func TestKubernetesClusterRoleBindingCreate_InvalidManifest(t *testing.T) {
	mock := NewMockServer(t)

	r := resourceKubernetesClusterRoleBindings()
	d := r.TestResourceData()
	_ = d.Set("endpoint_id", 1)
	_ = d.Set("manifest", "::: not valid")

	if err := rcCreate(r, d, mock.Client()); err == nil {
		t.Fatal("expected error for invalid manifest, got nil")
	}
}

// TestKubernetesClusterRoleBindingDelete_HappyPath verifies DELETE.
func TestKubernetesClusterRoleBindingDelete_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("DELETE", "/endpoints/1/kubernetes/apis/rbac.authorization.k8s.io/v1/clusterrolebindings/global-admin",
		RespondString(http.StatusOK, "", ""))

	r := resourceKubernetesClusterRoleBindings()
	d := r.TestResourceData()
	d.SetId("1:global-admin")

	if err := rcDelete(r, d, mock.Client()); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	if mock.FindRequest("DELETE", "/endpoints/1/kubernetes/apis/rbac.authorization.k8s.io/v1/clusterrolebindings/global-admin") == nil {
		t.Error("expected DELETE request to be recorded")
	}
}

// TestKubernetesClusterRoleBindingParseID verifies ID parsing.
func TestKubernetesClusterRoleBindingParseID(t *testing.T) {
	endpointID, name := parseClusterRolesBindingsID("11:bind-x")
	if endpointID != 11 || name != "bind-x" {
		t.Errorf("expected (11, bind-x), got (%d, %q)", endpointID, name)
	}
}
