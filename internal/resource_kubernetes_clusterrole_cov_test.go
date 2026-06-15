package internal

import (
	"net/http"
	"testing"
)

// TestKubernetesClusterRoleCreate_InvalidManifest covers the parse error branch.
func TestKubernetesClusterRoleCreate_InvalidManifest(t *testing.T) {
	mock := NewMockServer(t)

	r := resourceKubernetesClusterRoles()
	d := r.TestResourceData()
	_ = d.Set("endpoint_id", 1)
	_ = d.Set("manifest", "::: not valid :::")

	if err := rcCreate(r, d, mock.Client()); err == nil {
		t.Fatal("expected error for invalid manifest, got nil")
	}
}

// TestKubernetesClusterRoleCreate_MissingMetadata covers the missing-metadata branch.
func TestKubernetesClusterRoleCreate_MissingMetadata(t *testing.T) {
	mock := NewMockServer(t)

	r := resourceKubernetesClusterRoles()
	d := r.TestResourceData()
	_ = d.Set("endpoint_id", 1)
	_ = d.Set("manifest", `{"kind":"ClusterRole"}`)

	if err := rcCreate(r, d, mock.Client()); err == nil {
		t.Fatal("expected error for missing metadata, got nil")
	}
}

// TestKubernetesClusterRoleCreate_MissingName covers the missing metadata.name branch.
func TestKubernetesClusterRoleCreate_MissingName(t *testing.T) {
	mock := NewMockServer(t)

	r := resourceKubernetesClusterRoles()
	d := r.TestResourceData()
	_ = d.Set("endpoint_id", 1)
	_ = d.Set("manifest", `{"kind":"ClusterRole","metadata":{}}`)

	if err := rcCreate(r, d, mock.Client()); err == nil {
		t.Fatal("expected error for missing metadata.name, got nil")
	}
}

// TestKubernetesClusterRoleReadNoop covers the no-op Read handler.
func TestKubernetesClusterRoleReadNoop(t *testing.T) {
	mock := NewMockServer(t)

	r := resourceKubernetesClusterRoles()
	d := r.TestResourceData()
	d.SetId("1:cluster-reader")

	if err := rcRead(r, d, mock.Client()); err != nil {
		t.Fatalf("Read noop should not error: %v", err)
	}
	if d.Id() != "1:cluster-reader" {
		t.Errorf("expected ID untouched by noop read, got %q", d.Id())
	}
}

// TestKubernetesClusterRoleDelete_404IsSuccess covers the 404-tolerant delete branch.
func TestKubernetesClusterRoleDelete_404IsSuccess(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("DELETE", "/endpoints/1/kubernetes/apis/rbac.authorization.k8s.io/v1/clusterroles/cluster-reader",
		RespondString(http.StatusNotFound, "application/json", `{"message":"gone"}`))

	r := resourceKubernetesClusterRoles()
	d := r.TestResourceData()
	d.SetId("1:cluster-reader")

	if err := rcDelete(r, d, mock.Client()); err != nil {
		t.Fatalf("Delete should treat 404 as success, got: %v", err)
	}
	if d.Id() != "" {
		t.Errorf("expected ID cleared, got %q", d.Id())
	}
}

// TestKubernetesClusterRoleDelete_HTTPError covers the delete error branch.
func TestKubernetesClusterRoleDelete_HTTPError(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("DELETE", "/endpoints/1/kubernetes/apis/rbac.authorization.k8s.io/v1/clusterroles/cluster-reader",
		RespondString(http.StatusInternalServerError, "application/json", `{"message":"boom"}`))

	r := resourceKubernetesClusterRoles()
	d := r.TestResourceData()
	d.SetId("1:cluster-reader")

	if err := rcDelete(r, d, mock.Client()); err == nil {
		t.Fatal("expected error on DELETE 500, got nil")
	}
}

// TestKubernetesClusterRoleUpdate_HappyPath covers Update which performs a
// Delete followed by a Create.
func TestKubernetesClusterRoleUpdate_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("DELETE", "/endpoints/1/kubernetes/apis/rbac.authorization.k8s.io/v1/clusterroles/cluster-reader",
		RespondString(http.StatusOK, "", ""))
	mock.On("POST", "/endpoints/1/kubernetes/apis/rbac.authorization.k8s.io/v1/clusterroles",
		RespondJSON(http.StatusCreated, map[string]interface{}{"kind": "ClusterRole"}))

	r := resourceKubernetesClusterRoles()
	d := r.TestResourceData()
	d.SetId("1:cluster-reader")
	_ = d.Set("endpoint_id", 1)
	_ = d.Set("manifest", clusterRoleManifestJSON)

	if err := rcUpdate(r, d, mock.Client()); err != nil {
		t.Fatalf("Update failed: %v", err)
	}
	if mock.FindRequest("DELETE", "/endpoints/1/kubernetes/apis/rbac.authorization.k8s.io/v1/clusterroles/cluster-reader") == nil {
		t.Error("expected DELETE during update")
	}
	if mock.FindRequest("POST", "/endpoints/1/kubernetes/apis/rbac.authorization.k8s.io/v1/clusterroles") == nil {
		t.Error("expected POST during update")
	}
	if d.Id() != "1:cluster-reader" {
		t.Errorf("expected ID re-set after update, got %q", d.Id())
	}
}

// TestKubernetesClusterRoleParseID_ThreeParts covers the SplitN cap at 3 parts.
func TestKubernetesClusterRoleParseID_ThreeParts(t *testing.T) {
	endpointID, name := parseClusterRolesID("2:foo:bar")
	if endpointID != 2 || name != "foo" {
		t.Errorf("expected (2, foo), got (%d, %q)", endpointID, name)
	}
}
