package internal

import (
	"net/http"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
)

// TestUserCreate_HappyPath exercises the create path for a brand new user.
// The resource lists users first, finds no match, POSTs, then re-reads via
// UserInspect and TeamMembershipList.
func TestUserCreate_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	// Initial list — empty, user does not exist yet.
	mock.On("GET", "/users", RespondJSON(http.StatusOK, []map[string]interface{}{}))

	mock.On("POST", "/users", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id":       42,
		"Username": "alice",
		"Role":     2,
	}))

	// Re-read after create.
	mock.On("GET", "/users/42", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id":       42,
		"Username": "alice",
		"Role":     2,
	}))
	// Team membership scan (Role=2 triggers a list).
	mock.On("GET", "/team_memberships", RespondJSON(http.StatusOK, []map[string]interface{}{}))

	r := resourceUser()
	d := r.TestResourceData()
	_ = d.Set("username", "alice")
	_ = d.Set("password", "secret-pw")
	_ = d.Set("role", 2)

	if err := rcCreate(r, d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if d.Id() != "42" {
		t.Errorf("expected ID %q, got %q", "42", d.Id())
	}
	if got := d.Get("username"); got != "alice" {
		t.Errorf("username: expected %q, got %v", "alice", got)
	}
	if got := d.Get("role"); got != 2 {
		t.Errorf("role: expected 2, got %v", got)
	}

	// Verify POST payload.
	post := mock.FindRequest("POST", "/users")
	if post == nil {
		t.Fatal("expected POST /users")
	}
	var payload map[string]interface{}
	if err := post.DecodeJSON(&payload); err != nil {
		t.Fatalf("decode POST: %v", err)
	}
	if got := payload["username"]; got != "alice" {
		t.Errorf("payload.username: expected %q, got %v", "alice", got)
	}
	if got := payload["password"]; got != "secret-pw" {
		t.Errorf("payload.password: expected %q, got %v", "secret-pw", got)
	}
	if got := payload["role"]; got != float64(2) {
		t.Errorf("payload.role: expected 2, got %v", got)
	}
}

// TestUserCreate_LDAP verifies that an LDAP user is created without a
// password field in the payload.
func TestUserCreate_LDAP(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/users", RespondJSON(http.StatusOK, []map[string]interface{}{}))
	mock.On("POST", "/users", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id":       55,
		"Username": "ldap-bob",
		"Role":     1,
	}))
	mock.On("GET", "/users/55", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id":       55,
		"Username": "ldap-bob",
		"Role":     1,
	}))

	r := resourceUser()
	d := r.TestResourceData()
	_ = d.Set("username", "ldap-bob")
	_ = d.Set("ldap_user", true)
	_ = d.Set("role", 1)

	if err := rcCreate(r, d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if d.Id() != "55" {
		t.Errorf("expected ID %q, got %q", "55", d.Id())
	}

	post := mock.FindRequest("POST", "/users")
	if post == nil {
		t.Fatal("expected POST /users")
	}
	var payload map[string]interface{}
	if err := post.DecodeJSON(&payload); err != nil {
		t.Fatalf("decode POST: %v", err)
	}
	// For LDAP users, the resource intentionally leaves the password pointer
	// nil. The JSON tag has no omitempty so the field still appears, but it
	// must serialize as a JSON null (not a real password string).
	if got, ok := payload["password"]; ok && got != nil {
		t.Errorf("LDAP user payload password should be null, got: %v", got)
	}
}

// TestUserCreate_LDAPWithPasswordRejected verifies the early-exit guard.
func TestUserCreate_LDAPWithPasswordRejected(t *testing.T) {
	mock := NewMockServer(t)
	r := resourceUser()
	d := r.TestResourceData()
	_ = d.Set("username", "x")
	_ = d.Set("ldap_user", true)
	_ = d.Set("password", "should-not-be-set")

	err := rcCreate(r, d, mock.Client())
	if err == nil {
		t.Fatal("expected error when password is set for LDAP user, got nil")
	}
}

// TestUserCreate_NoPasswordRejected verifies the non-LDAP guard.
func TestUserCreate_NoPasswordRejected(t *testing.T) {
	mock := NewMockServer(t)
	r := resourceUser()
	d := r.TestResourceData()
	_ = d.Set("username", "no-pw")
	_ = d.Set("ldap_user", false)
	// password intentionally empty

	err := rcCreate(r, d, mock.Client())
	if err == nil {
		t.Fatal("expected error when password is empty for non-LDAP user, got nil")
	}
}

// TestUserRead_HappyPath verifies state population from UserInspect.
func TestUserRead_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/users/7", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id":       7,
		"Username": "charlie",
		"Role":     2,
	}))
	mock.On("GET", "/team_memberships", RespondJSON(http.StatusOK, []map[string]interface{}{
		{"Id": 1, "Role": 2, "TeamID": 33, "UserID": 7},
	}))

	r := resourceUser()
	d := r.TestResourceData()
	d.SetId("7")
	_ = d.Set("team_id", 33)

	if err := rcRead(r, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if got := d.Get("username"); got != "charlie" {
		t.Errorf("username: expected %q, got %v", "charlie", got)
	}
	if got := d.Get("role"); got != 2 {
		t.Errorf("role: expected 2, got %v", got)
	}
	if got := d.Get("team_id"); got != 33 {
		t.Errorf("team_id: expected 33, got %v", got)
	}
}

// TestUserRead_TeamIDTracksOnlyManagedTeam is the regression test for a
// team_id picked up from any membership: a user added to a team with
// portainer_team_membership planned "team_id = 1 -> null" on every run.
func TestUserRead_TeamIDTracksOnlyManagedTeam(t *testing.T) {
	cases := []struct {
		name  string
		state int
		want  int
	}{
		{"not managed", 0, 0},
		{"managed and present", 33, 33},
		{"managed and removed outside Terraform", 44, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mock := NewMockServer(t)
			mock.On("GET", "/users/7", RespondJSON(http.StatusOK, map[string]interface{}{"Id": 7, "Username": "charlie", "Role": 2}))
			mock.On("GET", "/team_memberships", RespondJSON(http.StatusOK, []map[string]interface{}{
				{"Id": 1, "Role": 1, "TeamID": 33, "UserID": 7},
			}))

			r := resourceUser()
			d := r.TestResourceData()
			d.SetId("7")
			_ = d.Set("team_id", tc.state)

			if err := rcRead(r, d, mock.Client()); err != nil {
				t.Fatalf("Read failed: %v", err)
			}
			if got := d.Get("team_id"); got != tc.want {
				t.Errorf("team_id: expected %d, got %v", tc.want, got)
			}
		})
	}
}

// TestUserUpdate_MovesTeam verifies a team_id change, which Update used to
// ignore, removes the old membership and adds the new one.
func TestUserUpdate_MovesTeam(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("PUT", "/users/7", RespondJSON(http.StatusOK, map[string]interface{}{}))
	mock.On("GET", "/users/7", RespondJSON(http.StatusOK, map[string]interface{}{"Id": 7, "Username": "charlie", "Role": 2}))
	mock.On("GET", "/team_memberships", RespondJSON(http.StatusOK, []map[string]interface{}{
		{"Id": 5, "Role": 2, "TeamID": 33, "UserID": 7},
	}))
	mock.On("DELETE", "/team_memberships/5", RespondString(http.StatusNoContent, "", ""))
	mock.On("POST", "/team_memberships", RespondJSON(http.StatusOK, map[string]interface{}{"Id": 6}))

	r := resourceUser()
	state := &terraform.InstanceState{ID: "7", Attributes: map[string]string{
		"id": "7", "username": "charlie", "role": "2", "team_id": "33",
	}}
	diff := &terraform.InstanceDiff{Attributes: map[string]*terraform.ResourceAttrDiff{
		"team_id": {Old: "33", New: "44"},
	}}
	d, err := schema.InternalMap(r.SchemaMap()).Data(state, diff)
	if err != nil {
		t.Fatalf("build data: %v", err)
	}

	// Read after Update no longer finds team 44 in the static mock; only the
	// requests matter here.
	_ = rcUpdate(r, d, mock.Client())

	if mock.FindRequest("DELETE", "/team_memberships/5") == nil {
		t.Error("expected the membership in team 33 to be removed")
	}
	post := mock.FindRequest("POST", "/team_memberships")
	if post == nil {
		t.Fatal("expected a membership in team 44 to be created")
	}
	var payload map[string]int
	if err := post.DecodeJSON(&payload); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if payload["userID"] != 7 || payload["teamID"] != 44 || payload["role"] != 2 {
		t.Errorf("payload: expected user 7 in team 44 as member, got %v", payload)
	}
}

// TestUserRead_404_ClearsID verifies drift detection.
func TestUserRead_404_ClearsID(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/users/404", RespondString(
		http.StatusNotFound, "application/json",
		`{"message":"user not found"}`,
	))

	r := resourceUser()
	d := r.TestResourceData()
	d.SetId("404")

	if err := rcRead(r, d, mock.Client()); err != nil {
		t.Fatalf("Read should swallow 404, got: %v", err)
	}
	if d.Id() != "" {
		t.Errorf("expected ID cleared, got %q", d.Id())
	}
}

// TestUserDelete_HappyPath verifies DELETE is sent.
func TestUserDelete_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("DELETE", "/users/13", RespondString(http.StatusNoContent, "", ""))

	r := resourceUser()
	d := r.TestResourceData()
	d.SetId("13")

	if err := rcDelete(r, d, mock.Client()); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	if mock.FindRequest("DELETE", "/users/13") == nil {
		t.Error("expected DELETE /users/13 to be sent")
	}
}

// TestUserUpdate_HappyPath verifies the PUT /users/{id} call is sent
// (without a password change since old password is unknown).
func TestUserUpdate_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("PUT", "/users/6", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id":       6,
		"Username": "renamed",
		"Role":     2,
	}))
	mock.On("GET", "/users/6", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id":       6,
		"Username": "renamed",
		"Role":     2,
	}))
	mock.On("GET", "/team_memberships", RespondJSON(http.StatusOK, []map[string]interface{}{}))

	r := resourceUser()
	d := r.TestResourceData()
	d.SetId("6")
	_ = d.Set("username", "renamed")
	_ = d.Set("role", 2)

	if err := rcUpdate(r, d, mock.Client()); err != nil {
		t.Fatalf("Update failed: %v", err)
	}
	put := mock.FindRequest("PUT", "/users/6")
	if put == nil {
		t.Fatal("expected PUT /users/6")
	}
	var payload map[string]interface{}
	if err := put.DecodeJSON(&payload); err != nil {
		t.Fatalf("decode PUT: %v", err)
	}
	if got := payload["username"]; got != "renamed" {
		t.Errorf("payload.username: expected %q, got %v", "renamed", got)
	}
	if got := payload["useCache"]; got != true {
		t.Errorf("payload.useCache: expected true, got %v", got)
	}
}
