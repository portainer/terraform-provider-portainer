package internal

import (
	"net/http"
	"testing"
)

const roleManifestJSON = `{
  "apiVersion": "rbac.authorization.k8s.io/v1",
  "kind": "Role",
  "metadata": {"name": "pod-reader"},
  "rules": [{"apiGroups": [""], "resources": ["pods"], "verbs": ["get","list"]}]
}`

// TestKubernetesRoleCreate_HappyPath verifies POST to RBAC roles endpoint and ID.
func TestKubernetesRoleCreate_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("POST", "/endpoints/1/kubernetes/apis/rbac.authorization.k8s.io/v1/namespaces/default/roles",
		RespondJSON(http.StatusCreated, map[string]interface{}{"kind": "Role"}))

	r := resourceKubernetesRoles()
	d := r.TestResourceData()
	_ = d.Set("endpoint_id", 1)
	_ = d.Set("namespace", "default")
	_ = d.Set("manifest", roleManifestJSON)

	if err := rcCreate(r, d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if d.Id() != "1:default:pod-reader" {
		t.Errorf("expected ID %q, got %q", "1:default:pod-reader", d.Id())
	}

	post := mock.FindRequest("POST", "/endpoints/1/kubernetes/apis/rbac.authorization.k8s.io/v1/namespaces/default/roles")
	if post == nil {
		t.Fatal("expected POST request to be recorded")
	}
	var payload map[string]interface{}
	if err := post.DecodeJSON(&payload); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if payload["kind"] != "Role" {
		t.Errorf("payload.kind: expected %q, got %v", "Role", payload["kind"])
	}
}

// TestKubernetesRoleCreate_HTTPError verifies HTTP error surfaces.
func TestKubernetesRoleCreate_HTTPError(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("POST", "/endpoints/1/kubernetes/apis/rbac.authorization.k8s.io/v1/namespaces/default/roles",
		RespondString(http.StatusForbidden, "application/json", `{"message":"forbidden"}`))

	r := resourceKubernetesRoles()
	d := r.TestResourceData()
	_ = d.Set("endpoint_id", 1)
	_ = d.Set("namespace", "default")
	_ = d.Set("manifest", roleManifestJSON)

	if err := rcCreate(r, d, mock.Client()); err == nil {
		t.Fatal("expected error on HTTP 403, got nil")
	}
}

// TestKubernetesRoleDelete_HappyPath verifies DELETE is sent.
func TestKubernetesRoleDelete_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("DELETE", "/endpoints/1/kubernetes/apis/rbac.authorization.k8s.io/v1/namespaces/default/roles/pod-reader",
		RespondString(http.StatusOK, "", ""))

	r := resourceKubernetesRoles()
	d := r.TestResourceData()
	d.SetId("1:default:pod-reader")

	if err := rcDelete(r, d, mock.Client()); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	if mock.FindRequest("DELETE", "/endpoints/1/kubernetes/apis/rbac.authorization.k8s.io/v1/namespaces/default/roles/pod-reader") == nil {
		t.Error("expected DELETE request to be recorded")
	}
	if d.Id() != "" {
		t.Errorf("expected ID cleared, got %q", d.Id())
	}
}

// TestKubernetesRoleParseID verifies ID parsing.
func TestKubernetesRoleParseID(t *testing.T) {
	endpointID, namespace, name := parseRolesID("5:dev:reader")
	if endpointID != 5 || namespace != "dev" || name != "reader" {
		t.Errorf("expected (5, dev, reader), got (%d, %q, %q)", endpointID, namespace, name)
	}
}
