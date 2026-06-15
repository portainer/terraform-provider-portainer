package internal

import (
	"net/http"
	"testing"
)

const clusterRoleManifestJSON = `{
  "apiVersion": "rbac.authorization.k8s.io/v1",
  "kind": "ClusterRole",
  "metadata": {"name": "cluster-reader"},
  "rules": [{"apiGroups": [""], "resources": ["nodes"], "verbs": ["get","list"]}]
}`

// TestKubernetesClusterRoleCreate_HappyPath verifies POST and ID "<endpoint>:<name>".
func TestKubernetesClusterRoleCreate_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("POST", "/endpoints/1/kubernetes/apis/rbac.authorization.k8s.io/v1/clusterroles",
		RespondJSON(http.StatusCreated, map[string]interface{}{"kind": "ClusterRole"}))

	r := resourceKubernetesClusterRoles()
	d := r.TestResourceData()
	_ = d.Set("endpoint_id", 1)
	_ = d.Set("manifest", clusterRoleManifestJSON)

	if err := rcCreate(r, d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if d.Id() != "1:cluster-reader" {
		t.Errorf("expected ID %q, got %q", "1:cluster-reader", d.Id())
	}

	post := mock.FindRequest("POST", "/endpoints/1/kubernetes/apis/rbac.authorization.k8s.io/v1/clusterroles")
	if post == nil {
		t.Fatal("expected POST request to be recorded")
	}
	var payload map[string]interface{}
	if err := post.DecodeJSON(&payload); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if payload["kind"] != "ClusterRole" {
		t.Errorf("payload.kind: expected %q, got %v", "ClusterRole", payload["kind"])
	}
}

// TestKubernetesClusterRoleCreate_HTTPError verifies HTTP error surfaces.
func TestKubernetesClusterRoleCreate_HTTPError(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("POST", "/endpoints/1/kubernetes/apis/rbac.authorization.k8s.io/v1/clusterroles",
		RespondString(http.StatusForbidden, "application/json", `{"message":"nope"}`))

	r := resourceKubernetesClusterRoles()
	d := r.TestResourceData()
	_ = d.Set("endpoint_id", 1)
	_ = d.Set("manifest", clusterRoleManifestJSON)

	if err := rcCreate(r, d, mock.Client()); err == nil {
		t.Fatal("expected error on HTTP 403, got nil")
	}
}

// TestKubernetesClusterRoleDelete_HappyPath verifies DELETE.
func TestKubernetesClusterRoleDelete_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("DELETE", "/endpoints/1/kubernetes/apis/rbac.authorization.k8s.io/v1/clusterroles/cluster-reader",
		RespondString(http.StatusOK, "", ""))

	r := resourceKubernetesClusterRoles()
	d := r.TestResourceData()
	d.SetId("1:cluster-reader")

	if err := rcDelete(r, d, mock.Client()); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	if mock.FindRequest("DELETE", "/endpoints/1/kubernetes/apis/rbac.authorization.k8s.io/v1/clusterroles/cluster-reader") == nil {
		t.Error("expected DELETE request to be recorded")
	}
}

// TestKubernetesClusterRoleParseID verifies ID parsing.
func TestKubernetesClusterRoleParseID(t *testing.T) {
	endpointID, name := parseClusterRolesID("7:admin")
	if endpointID != 7 || name != "admin" {
		t.Errorf("expected (7, admin), got (%d, %q)", endpointID, name)
	}
	// malformed
	endpointID, name = parseClusterRolesID("bad")
	if endpointID != 0 || name != "" {
		t.Errorf("expected zero values on malformed ID, got (%d, %q)", endpointID, name)
	}
}
