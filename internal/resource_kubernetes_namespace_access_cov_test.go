package internal

import (
	"net/http"
	"testing"
)

// TestK8sAccessUpdate_ResolvesNamespaceName covers the full Update/Create path
// where namespace_id has no colon, forcing a name lookup via getNamespaceRPN,
// followed by the PUT to the pools access endpoint.
func TestK8sAccessUpdate_ResolvesNamespaceName(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/kubernetes/2/namespaces", RespondJSON(http.StatusOK, []map[string]interface{}{
		{"Name": "team-a", "Id": "abc"},
		{"Name": "team-b", "Id": "def"},
	}))
	mock.On("PUT", "/endpoints/2/pools/team-b/access", RespondString(http.StatusNoContent, "", ""))

	r := resourceKubernetesNamespaceAccess()
	d := r.TestResourceData()
	_ = d.Set("endpoint_id", 2)
	_ = d.Set("namespace_id", "team-b")
	_ = d.Set("users_to_add", []interface{}{1, 2})
	_ = d.Set("teams_to_remove", []interface{}{5})

	if err := rcCreate(r, d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if d.Id() != "2/team-b" {
		t.Errorf("expected ID %q, got %q", "2/team-b", d.Id())
	}

	put := mock.FindRequest("PUT", "/endpoints/2/pools/team-b/access")
	if put == nil {
		t.Fatal("expected PUT to pools access endpoint")
	}
	var payload map[string]interface{}
	if err := put.DecodeJSON(&payload); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	users, ok := payload["usersToAdd"].([]interface{})
	if !ok || len(users) != 2 {
		t.Errorf("usersToAdd: expected 2 entries, got %v", payload["usersToAdd"])
	}
}

// TestK8sAccessUpdate_ColonSkipsLookup covers the branch where namespace_id
// already contains a colon (RPN form) so no namespace listing is performed.
func TestK8sAccessUpdate_ColonSkipsLookup(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("PUT", "/endpoints/3/pools/ns:1234/access", RespondString(http.StatusNoContent, "", ""))

	r := resourceKubernetesNamespaceAccess()
	d := r.TestResourceData()
	_ = d.Set("endpoint_id", 3)
	_ = d.Set("namespace_id", "ns:1234")

	if err := rcUpdate(r, d, mock.Client()); err != nil {
		t.Fatalf("Update failed: %v", err)
	}
	if d.Id() != "3/ns:1234" {
		t.Errorf("expected ID %q, got %q", "3/ns:1234", d.Id())
	}
	if mock.FindRequest("GET", "/kubernetes/3/namespaces") != nil {
		t.Error("expected no namespace listing when namespace_id has a colon")
	}
}

// TestK8sAccessUpdate_NamespaceNotFound covers the error when the named
// namespace is absent from the listing.
func TestK8sAccessUpdate_NamespaceNotFound(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/kubernetes/1/namespaces", RespondJSON(http.StatusOK, []map[string]interface{}{
		{"Name": "other", "Id": "x"},
	}))

	r := resourceKubernetesNamespaceAccess()
	d := r.TestResourceData()
	_ = d.Set("endpoint_id", 1)
	_ = d.Set("namespace_id", "missing")

	if err := rcCreate(r, d, mock.Client()); err == nil {
		t.Fatal("expected error when namespace not found, got nil")
	}
}

// TestK8sAccessUpdate_ListError covers getNamespaceRPN's non-200 branch.
func TestK8sAccessUpdate_ListError(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/kubernetes/1/namespaces", RespondString(
		http.StatusInternalServerError, "application/json", `{"message":"boom"}`))

	r := resourceKubernetesNamespaceAccess()
	d := r.TestResourceData()
	_ = d.Set("endpoint_id", 1)
	_ = d.Set("namespace_id", "team")

	if err := rcCreate(r, d, mock.Client()); err == nil {
		t.Fatal("expected error when namespace listing fails, got nil")
	}
}

// TestK8sAccessUpdate_PutError covers the PUT non-204 branch.
func TestK8sAccessUpdate_PutError(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("PUT", "/endpoints/4/pools/ns:9/access", RespondString(
		http.StatusBadRequest, "application/json", `{"message":"bad"}`))

	r := resourceKubernetesNamespaceAccess()
	d := r.TestResourceData()
	_ = d.Set("endpoint_id", 4)
	_ = d.Set("namespace_id", "ns:9")

	if err := rcUpdate(r, d, mock.Client()); err == nil {
		t.Fatal("expected error on PUT 400, got nil")
	}
}

// TestK8sAccessReadDeleteNoop covers the no-op Read and Delete handlers.
func TestK8sAccessReadDeleteNoop(t *testing.T) {
	mock := NewMockServer(t)

	r := resourceKubernetesNamespaceAccess()
	d := r.TestResourceData()
	d.SetId("1/ns")

	if err := rcRead(r, d, mock.Client()); err != nil {
		t.Fatalf("Read noop should not error: %v", err)
	}
	if err := rcDelete(r, d, mock.Client()); err != nil {
		t.Fatalf("Delete noop should not error: %v", err)
	}
	// Delete noop does NOT clear the ID.
	if d.Id() != "1/ns" {
		t.Errorf("expected ID untouched by noop delete, got %q", d.Id())
	}
}
