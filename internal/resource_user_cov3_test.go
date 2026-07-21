package internal

import (
	"context"
	"net/http"
	"testing"
)

// =========================================================================
// cov3 coverage for resource_user.go focused on resourceUserCreate error and
// branch paths (list error, create HTTP error, team_id assignment happy path,
// team_id-with-wrong-role guard, generate_api_key-without-password guard) plus
// importer happy and error branches (numeric read error, list error) not
// exercised by resource_user_test.go / resource_user_cov_test.go.
// =========================================================================

// TestUserCov3_Create_ListHTTPError covers the branch where the initial
// UserList call fails (HTTP 500), so Create errors before any POST.
func TestUserCov3_Create_ListHTTPError(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/users", RespondString(
		http.StatusInternalServerError, "application/json", `{"message":"list boom"}`,
	))

	r := resourceUser()
	d := r.TestResourceData()
	_ = d.Set("username", "alice")
	_ = d.Set("password", "pw")

	if err := rcCreate(r, d, mock.Client()); err == nil {
		t.Fatal("expected error when UserList returns 500, got nil")
	}
	if d.Id() != "" {
		t.Errorf("expected empty ID after list error, got %q", d.Id())
	}
	if mock.FindRequest("POST", "/users") != nil {
		t.Error("did not expect POST /users after list failure")
	}
}

// TestUserCov3_Create_HTTPError covers the UserCreate error branch: the list
// succeeds (empty) but the POST fails with HTTP 500.
func TestUserCov3_Create_HTTPError(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/users", RespondJSON(http.StatusOK, []map[string]interface{}{}))
	mock.On("POST", "/users", RespondString(
		http.StatusInternalServerError, "application/json", `{"message":"create boom"}`,
	))

	r := resourceUser()
	d := r.TestResourceData()
	_ = d.Set("username", "bob")
	_ = d.Set("password", "pw")

	if err := rcCreate(r, d, mock.Client()); err == nil {
		t.Fatal("expected error when UserCreate returns 500, got nil")
	}
	if d.Id() != "" {
		t.Errorf("expected empty ID after create error, got %q", d.Id())
	}
}

// TestUserCov3_Create_TeamIDAssigned covers the team_id branch: after a
// successful create the resource POSTs a team membership, then re-reads.
func TestUserCov3_Create_TeamIDAssigned(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/users", RespondJSON(http.StatusOK, []map[string]interface{}{}))
	mock.On("POST", "/users", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id": 88, "Username": "teamuser", "Role": 2,
	}))
	mock.On("POST", "/team_memberships", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id": 1, "UserID": 88, "TeamID": 5, "Role": 2,
	}))
	// Re-read after create.
	mock.On("GET", "/users/88", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id": 88, "Username": "teamuser", "Role": 2,
	}))
	mock.On("GET", "/team_memberships", RespondJSON(http.StatusOK, []map[string]interface{}{}))

	r := resourceUser()
	d := r.TestResourceData()
	_ = d.Set("username", "teamuser")
	_ = d.Set("password", "pw")
	_ = d.Set("role", 2)
	_ = d.Set("team_id", 5)

	if err := rcCreate(r, d, mock.Client()); err != nil {
		t.Fatalf("Create with team_id failed: %v", err)
	}
	if d.Id() != "88" {
		t.Errorf("expected ID 88, got %q", d.Id())
	}
	if mock.FindRequest("POST", "/team_memberships") == nil {
		t.Error("expected POST /team_memberships to assign the user to a team")
	}
}

// TestUserCov3_Create_TeamIDWrongRole covers the guard that rejects team_id for
// non-standard users (role != 2). The user is created first, then the guard
// fires.
func TestUserCov3_Create_TeamIDWrongRole(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/users", RespondJSON(http.StatusOK, []map[string]interface{}{}))
	mock.On("POST", "/users", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id": 89, "Username": "adminteam", "Role": 1,
	}))

	r := resourceUser()
	d := r.TestResourceData()
	_ = d.Set("username", "adminteam")
	_ = d.Set("password", "pw")
	_ = d.Set("role", 1)
	_ = d.Set("team_id", 9)

	if err := rcCreate(r, d, mock.Client()); err == nil {
		t.Fatal("expected error when team_id is set for a role=1 user, got nil")
	}
	if mock.FindRequest("POST", "/team_memberships") != nil {
		t.Error("did not expect a team membership POST for a rejected role")
	}
}

// TestUserCov3_Create_APIKeyNoPassword covers the generate_api_key guard: an
// LDAP user (no password) with generate_api_key=true is created but the key
// generation is rejected because password is empty.
func TestUserCov3_Create_APIKeyNoPassword(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/users", RespondJSON(http.StatusOK, []map[string]interface{}{}))
	mock.On("POST", "/users", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id": 90, "Username": "ldapkey", "Role": 2,
	}))

	r := resourceUser()
	d := r.TestResourceData()
	_ = d.Set("username", "ldapkey")
	_ = d.Set("ldap_user", true)
	_ = d.Set("generate_api_key", true)

	if err := rcCreate(r, d, mock.Client()); err == nil {
		t.Fatal("expected error generating an API key without a password, got nil")
	}
}

// TestUserCov3_Import_HappyPath covers the numeric-ID import path: the ID is
// parsed as numeric and the user is read directly.
func TestUserCov3_Import_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/users/44", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id": 44, "Username": "imported44", "Role": 1,
	}))

	r := resourceUser()
	d := r.TestResourceData()
	d.SetId("44")

	results, err := r.Importer.StateContext(context.Background(), d, mock.Client())
	if err != nil {
		t.Fatalf("import failed: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 imported state, got %d", len(results))
	}
	if got := results[0].Get("username"); got != "imported44" {
		t.Errorf("username: expected imported44, got %v", got)
	}
}

// TestUserCov3_Import_NumericReadError covers the numeric-ID import path where
// the subsequent Read returns a non-404 error, which must fail the import.
func TestUserCov3_Import_NumericReadError(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/users/45", RespondString(
		http.StatusInternalServerError, "application/json", `{"message":"read boom"}`,
	))

	r := resourceUser()
	d := r.TestResourceData()
	d.SetId("45")

	if _, err := r.Importer.StateContext(context.Background(), d, mock.Client()); err == nil {
		t.Fatal("expected import error when numeric-ID read returns 500, got nil")
	}
}

// TestUserCov3_Import_ListError covers the by-username import path where the
// UserList call fails, so the import errors before any match.
func TestUserCov3_Import_ListError(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/users", RespondString(
		http.StatusInternalServerError, "application/json", `{"message":"list boom"}`,
	))

	r := resourceUser()
	d := r.TestResourceData()
	d.SetId("ghostuser")

	if _, err := r.Importer.StateContext(context.Background(), d, mock.Client()); err == nil {
		t.Fatal("expected import error when username listing returns 500, got nil")
	}
}
