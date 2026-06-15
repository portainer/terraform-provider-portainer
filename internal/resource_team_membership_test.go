package internal

import (
	"net/http"
	"testing"
)

// TestTeamMembershipCreate_HappyPath exercises POST /team_memberships then
// the Read which lists all memberships and finds the matching one.
func TestTeamMembershipCreate_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("POST", "/team_memberships", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id":     11,
		"Role":   2,
		"TeamID": 5,
		"UserID": 7,
	}))
	mock.On("GET", "/team_memberships", RespondJSON(http.StatusOK, []map[string]interface{}{
		{"Id": 11, "Role": 2, "TeamID": 5, "UserID": 7},
	}))

	r := resourceTeamMembership()
	d := r.TestResourceData()
	_ = d.Set("role", 2)
	_ = d.Set("team_id", 5)
	_ = d.Set("user_id", 7)

	if err := rcCreate(r, d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	if d.Id() != "11" {
		t.Errorf("expected ID %q, got %q", "11", d.Id())
	}
	if got := d.Get("role"); got != 2 {
		t.Errorf("role: expected 2, got %v", got)
	}
	if got := d.Get("team_id"); got != 5 {
		t.Errorf("team_id: expected 5, got %v", got)
	}
	if got := d.Get("user_id"); got != 7 {
		t.Errorf("user_id: expected 7, got %v", got)
	}

	// Verify payload field names (camelCase per swagger model).
	post := mock.FindRequest("POST", "/team_memberships")
	if post == nil {
		t.Fatal("expected POST /team_memberships")
	}
	var payload map[string]interface{}
	if err := post.DecodeJSON(&payload); err != nil {
		t.Fatalf("decode POST: %v", err)
	}
	if got := payload["role"]; got != float64(2) {
		t.Errorf("payload.role: expected 2, got %v", got)
	}
	if got := payload["teamID"]; got != float64(5) {
		t.Errorf("payload.teamID: expected 5, got %v", got)
	}
	if got := payload["userID"]; got != float64(7) {
		t.Errorf("payload.userID: expected 7, got %v", got)
	}
}

// TestTeamMembershipRead_NotInList verifies that a missing membership ID
// clears the resource ID.
func TestTeamMembershipRead_NotInList(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/team_memberships", RespondJSON(http.StatusOK, []map[string]interface{}{
		{"Id": 1, "Role": 1, "TeamID": 1, "UserID": 1},
	}))

	r := resourceTeamMembership()
	d := r.TestResourceData()
	d.SetId("999")

	if err := rcRead(r, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if d.Id() != "" {
		t.Errorf("expected ID cleared, got %q", d.Id())
	}
}

// TestTeamMembershipUpdate_HappyPath verifies the PUT call is sent with the
// expected payload.
func TestTeamMembershipUpdate_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("PUT", "/team_memberships/3", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id":     3,
		"Role":   1,
		"TeamID": 8,
		"UserID": 9,
	}))
	mock.On("GET", "/team_memberships", RespondJSON(http.StatusOK, []map[string]interface{}{
		{"Id": 3, "Role": 1, "TeamID": 8, "UserID": 9},
	}))

	r := resourceTeamMembership()
	d := r.TestResourceData()
	d.SetId("3")
	_ = d.Set("role", 1)
	_ = d.Set("team_id", 8)
	_ = d.Set("user_id", 9)

	if err := rcUpdate(r, d, mock.Client()); err != nil {
		t.Fatalf("Update failed: %v", err)
	}
	put := mock.FindRequest("PUT", "/team_memberships/3")
	if put == nil {
		t.Fatal("expected PUT /team_memberships/3")
	}
	var payload map[string]interface{}
	if err := put.DecodeJSON(&payload); err != nil {
		t.Fatalf("decode PUT: %v", err)
	}
	if got := payload["role"]; got != float64(1) {
		t.Errorf("payload.role: expected 1, got %v", got)
	}
}

// TestTeamMembershipDelete_HappyPath verifies the DELETE call is sent.
func TestTeamMembershipDelete_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("DELETE", "/team_memberships/22", RespondString(http.StatusNoContent, "", ""))

	r := resourceTeamMembership()
	d := r.TestResourceData()
	d.SetId("22")

	if err := rcDelete(r, d, mock.Client()); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	if mock.FindRequest("DELETE", "/team_memberships/22") == nil {
		t.Error("expected DELETE /team_memberships/22 to be sent")
	}
}

// TestTeamMembershipDelete_404_Idempotent verifies that a 404 on Delete is
// treated as success (resource was already gone).
func TestTeamMembershipDelete_404_Idempotent(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("DELETE", "/team_memberships/99", RespondString(
		http.StatusNotFound, "application/json",
		`{"message":"not found"}`,
	))

	r := resourceTeamMembership()
	d := r.TestResourceData()
	d.SetId("99")

	if err := rcDelete(r, d, mock.Client()); err != nil {
		t.Fatalf("Delete should treat 404 as success, got: %v", err)
	}
}
