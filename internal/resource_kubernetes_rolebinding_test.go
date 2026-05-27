package internal

import (
	"net/http"
	"testing"
)

const roleBindingManifestJSON = `{
  "apiVersion": "rbac.authorization.k8s.io/v1",
  "kind": "RoleBinding",
  "metadata": {"name": "read-pods"},
  "subjects": [{"kind": "User", "name": "alice"}],
  "roleRef": {"kind": "Role", "name": "pod-reader", "apiGroup": "rbac.authorization.k8s.io"}
}`

// TestKubernetesRoleBindingCreate_HappyPath verifies POST and composite ID.
func TestKubernetesRoleBindingCreate_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("POST", "/endpoints/1/kubernetes/apis/rbac.authorization.k8s.io/v1/namespaces/default/rolebindings",
		RespondJSON(http.StatusCreated, map[string]interface{}{"kind": "RoleBinding"}))

	r := resourceKubernetesRoleBindings()
	d := r.TestResourceData()
	_ = d.Set("endpoint_id", 1)
	_ = d.Set("namespace", "default")
	_ = d.Set("manifest", roleBindingManifestJSON)

	if err := r.Create(d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if d.Id() != "1:default:read-pods" {
		t.Errorf("expected ID %q, got %q", "1:default:read-pods", d.Id())
	}

	post := mock.FindRequest("POST", "/endpoints/1/kubernetes/apis/rbac.authorization.k8s.io/v1/namespaces/default/rolebindings")
	if post == nil {
		t.Fatal("expected POST request to be recorded")
	}
	var payload map[string]interface{}
	if err := post.DecodeJSON(&payload); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if payload["kind"] != "RoleBinding" {
		t.Errorf("payload.kind: expected %q, got %v", "RoleBinding", payload["kind"])
	}
}

// TestKubernetesRoleBindingCreate_HTTPError verifies HTTP error surfaces.
func TestKubernetesRoleBindingCreate_HTTPError(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("POST", "/endpoints/1/kubernetes/apis/rbac.authorization.k8s.io/v1/namespaces/default/rolebindings",
		RespondString(http.StatusUnprocessableEntity, "application/json", `{"message":"invalid"}`))

	r := resourceKubernetesRoleBindings()
	d := r.TestResourceData()
	_ = d.Set("endpoint_id", 1)
	_ = d.Set("namespace", "default")
	_ = d.Set("manifest", roleBindingManifestJSON)

	if err := r.Create(d, mock.Client()); err == nil {
		t.Fatal("expected error on HTTP 422, got nil")
	}
}

// TestKubernetesRoleBindingDelete_HappyPath verifies DELETE.
func TestKubernetesRoleBindingDelete_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("DELETE", "/endpoints/1/kubernetes/apis/rbac.authorization.k8s.io/v1/namespaces/default/rolebindings/read-pods",
		RespondString(http.StatusOK, "", ""))

	r := resourceKubernetesRoleBindings()
	d := r.TestResourceData()
	d.SetId("1:default:read-pods")

	if err := r.Delete(d, mock.Client()); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	if mock.FindRequest("DELETE", "/endpoints/1/kubernetes/apis/rbac.authorization.k8s.io/v1/namespaces/default/rolebindings/read-pods") == nil {
		t.Error("expected DELETE request to be recorded")
	}
}

// TestKubernetesRoleBindingParseID verifies ID parsing.
func TestKubernetesRoleBindingParseID(t *testing.T) {
	endpointID, namespace, name := parseRoleBindingsID("9:team:bind")
	if endpointID != 9 || namespace != "team" || name != "bind" {
		t.Errorf("expected (9, team, bind), got (%d, %q, %q)", endpointID, namespace, name)
	}
}
