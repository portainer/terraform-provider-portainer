package internal

import (
	"net/http"
	"testing"
)

// =========================================================================
// cov2 coverage for resource_user.go: the generate_api_key happy path
// (authenticate as the new user, then generate an access token) which is the
// largest untested chunk of resourceUserCreate, plus the standard-user
// team-membership + api-key combination.
// =========================================================================

// TestUserCov2_Create_ApiKeyHappy covers the full generate_api_key branch: a
// non-LDAP user is created, the resource authenticates as that user
// (POST /auth), then generates an API key (POST /users/{id}/tokens) and stores
// the raw key in state. Finally Create chains into Read.
func TestUserCov2_Create_ApiKeyHappy(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/users", RespondJSON(http.StatusOK, []map[string]interface{}{}))
	mock.On("POST", "/users", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id": 100, "Username": "keyer", "Role": 2,
	}))
	// Authenticate as the freshly created user.
	mock.On("POST", "/auth", RespondJSON(http.StatusOK, map[string]interface{}{
		"jwt": "jwt-token-xyz",
	}))
	// Generate the API key.
	mock.On("POST", "/users/100/tokens", RespondJSON(http.StatusOK, map[string]interface{}{
		"rawAPIKey": "raw-key-abc123",
	}))
	// Create chains into Read.
	mock.On("GET", "/users/100", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id": 100, "Username": "keyer", "Role": 2,
	}))
	mock.On("GET", "/team_memberships", RespondJSON(http.StatusOK, []map[string]interface{}{}))

	r := resourceUser()
	d := r.TestResourceData()
	_ = d.Set("username", "keyer")
	_ = d.Set("password", "secret-pw")
	_ = d.Set("role", 2)
	_ = d.Set("generate_api_key", true)
	_ = d.Set("api_key_description", "tf-key")

	if err := rcCreate(r, d, mock.Client()); err != nil {
		t.Fatalf("Create with generate_api_key failed: %v", err)
	}
	if d.Id() != "100" {
		t.Errorf("expected ID 100, got %q", d.Id())
	}
	if got := d.Get("api_key_raw"); got != "raw-key-abc123" {
		t.Errorf("api_key_raw: expected %q, got %v", "raw-key-abc123", got)
	}

	if mock.FindRequest("POST", "/auth") == nil {
		t.Error("expected POST /auth to authenticate as the new user")
	}
	keyReq := mock.FindRequest("POST", "/users/100/tokens")
	if keyReq == nil {
		t.Fatal("expected POST /users/100/tokens to generate the API key")
	}
	var payload map[string]interface{}
	if err := keyReq.DecodeJSON(&payload); err != nil {
		t.Fatalf("decode token POST body: %v", err)
	}
	if got := payload["description"]; got != "tf-key" {
		t.Errorf("token payload.description: expected %q, got %v", "tf-key", got)
	}
	if got := payload["password"]; got != "secret-pw" {
		t.Errorf("token payload.password: expected the user password, got %v", got)
	}
}

// TestUserCov2_Create_TeamIDAndApiKey combines the standard-user team-assignment
// branch with the generate_api_key branch in a single Create so both follow-up
// call chains (team membership POST + auth/token POSTs) run.
func TestUserCov2_Create_TeamIDAndApiKey(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/users", RespondJSON(http.StatusOK, []map[string]interface{}{}))
	mock.On("POST", "/users", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id": 101, "Username": "teamkeyer", "Role": 2,
	}))
	mock.On("POST", "/team_memberships", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id": 9, "UserID": 101, "TeamID": 3, "Role": 2,
	}))
	mock.On("POST", "/auth", RespondJSON(http.StatusOK, map[string]interface{}{
		"jwt": "jwt-token-team",
	}))
	mock.On("POST", "/users/101/tokens", RespondJSON(http.StatusOK, map[string]interface{}{
		"rawAPIKey": "raw-team-key",
	}))
	mock.On("GET", "/users/101", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id": 101, "Username": "teamkeyer", "Role": 2,
	}))
	mock.On("GET", "/team_memberships", RespondJSON(http.StatusOK, []map[string]interface{}{}))

	r := resourceUser()
	d := r.TestResourceData()
	_ = d.Set("username", "teamkeyer")
	_ = d.Set("password", "pw")
	_ = d.Set("role", 2)
	_ = d.Set("team_id", 3)
	_ = d.Set("generate_api_key", true)

	if err := rcCreate(r, d, mock.Client()); err != nil {
		t.Fatalf("Create with team_id + generate_api_key failed: %v", err)
	}
	if d.Id() != "101" {
		t.Errorf("expected ID 101, got %q", d.Id())
	}
	if mock.FindRequest("POST", "/team_memberships") == nil {
		t.Error("expected POST /team_memberships for team assignment")
	}
	if mock.FindRequest("POST", "/users/101/tokens") == nil {
		t.Error("expected POST /users/101/tokens for API key generation")
	}
	if got := d.Get("api_key_raw"); got != "raw-team-key" {
		t.Errorf("api_key_raw: expected %q, got %v", "raw-team-key", got)
	}
}
