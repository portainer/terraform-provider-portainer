package internal

import (
	"context"
	"net/http"
	"testing"
)

// =========================================================================
// Additional coverage for resource_user.go: the existing-user create guard
// (delegates to Update), Delete 404 swallow, the import-by-numeric-ID and
// import-by-username paths, and the Read error (non-404) path.
// =========================================================================

// TestUserCreate_ExistingUserGuard verifies that when a user with the same
// username already exists, Create reuses the ID and delegates to Update.
func TestUserCreate_ExistingUserGuard(t *testing.T) {
	mock := NewMockServer(t)

	// List returns an existing user with the requested name.
	mock.On("GET", "/users", RespondJSON(http.StatusOK, []map[string]interface{}{
		{"Id": 70, "Username": "dup", "Role": 2},
	}))
	// Update path: PUT /users/70 then Read (UserInspect + team_memberships).
	mock.On("PUT", "/users/70", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id": 70, "Username": "dup", "Role": 2,
	}))
	mock.On("GET", "/users/70", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id": 70, "Username": "dup", "Role": 2,
	}))
	mock.On("GET", "/team_memberships", RespondJSON(http.StatusOK, []map[string]interface{}{}))

	r := resourceUser()
	d := r.TestResourceData()
	_ = d.Set("username", "dup")
	_ = d.Set("password", "pw")
	_ = d.Set("role", 2)

	if err := rcCreate(r, d, mock.Client()); err != nil {
		t.Fatalf("Create (existing-user guard) failed: %v", err)
	}
	if d.Id() != "70" {
		t.Errorf("expected reused ID 70, got %q", d.Id())
	}
	if mock.FindRequest("POST", "/users") != nil {
		t.Error("expected NO POST /users when username already exists")
	}
	if mock.FindRequest("PUT", "/users/70") == nil {
		t.Error("expected PUT /users/70 (Update delegation)")
	}
}

// TestUserDelete_404Swallowed verifies a NotFound on delete is treated as
// success.
func TestUserDelete_404Swallowed(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("DELETE", "/users/404", RespondString(
		http.StatusNotFound, "application/json", `{"message":"gone"}`,
	))

	r := resourceUser()
	d := r.TestResourceData()
	d.SetId("404")

	if err := rcDelete(r, d, mock.Client()); err != nil {
		t.Fatalf("Delete should swallow 404, got: %v", err)
	}
}

// TestUserImport_NumericID verifies the import path for a numeric ID, which
// just reads the user.
func TestUserImport_NumericID(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/users/12", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id": 12, "Username": "imported", "Role": 1,
	}))

	r := resourceUser()
	d := r.TestResourceData()
	d.SetId("12")

	out, err := r.Importer.StateContext(context.Background(), d, mock.Client())
	if err != nil {
		t.Fatalf("import failed: %v", err)
	}
	if len(out) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(out))
	}
	if got := out[0].Get("username"); got != "imported" {
		t.Errorf("username: expected imported, got %v", got)
	}
}

// TestUserImport_ByUsername verifies the import-by-username path: list users,
// match by name, then read.
func TestUserImport_ByUsername(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/users", RespondJSON(http.StatusOK, []map[string]interface{}{
		{"Id": 21, "Username": "byname", "Role": 1},
	}))
	mock.On("GET", "/users/21", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id": 21, "Username": "byname", "Role": 1,
	}))

	r := resourceUser()
	d := r.TestResourceData()
	d.SetId("byname")

	out, err := r.Importer.StateContext(context.Background(), d, mock.Client())
	if err != nil {
		t.Fatalf("import failed: %v", err)
	}
	if len(out) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(out))
	}
	if out[0].Id() != "21" {
		t.Errorf("expected resolved ID 21, got %q", out[0].Id())
	}
}

// TestUserImport_UsernameNotFound verifies import errors when the username is
// not present.
func TestUserImport_UsernameNotFound(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/users", RespondJSON(http.StatusOK, []map[string]interface{}{
		{"Id": 1, "Username": "someone", "Role": 1},
	}))

	r := resourceUser()
	d := r.TestResourceData()
	d.SetId("ghost")

	if _, err := r.Importer.StateContext(context.Background(), d, mock.Client()); err == nil {
		t.Fatal("expected error for unknown username on import, got nil")
	}
}
