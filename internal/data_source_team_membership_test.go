package internal

import (
	"net/http"
	"testing"
)

// TestDataSourceTeamMembershipRead_HappyPath verifies that the data source
// lists memberships, filters by team_id+user_id, and populates the role.
func TestDataSourceTeamMembershipRead_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/team_memberships", RespondJSON(http.StatusOK, []map[string]interface{}{
		{"Id": 10, "TeamID": 1, "UserID": 5, "Role": 1},
		{"Id": 11, "TeamID": 2, "UserID": 5, "Role": 2},
		{"Id": 12, "TeamID": 2, "UserID": 6, "Role": 2},
	}))

	ds := dataSourceTeamMembership()
	d := ds.TestResourceData()
	_ = d.Set("team_id", 2)
	_ = d.Set("user_id", 5)

	if err := rcRead(ds, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	if d.Id() != "11" {
		t.Errorf("expected ID %q, got %q", "11", d.Id())
	}
	if got := d.Get("role"); got != 2 {
		t.Errorf("role: expected 2, got %v", got)
	}
}

// TestDataSourceTeamMembershipRead_NotFound verifies the error path when no
// membership matches the filter.
func TestDataSourceTeamMembershipRead_NotFound(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/team_memberships", RespondJSON(http.StatusOK, []map[string]interface{}{
		{"Id": 10, "TeamID": 1, "UserID": 5, "Role": 1},
	}))

	ds := dataSourceTeamMembership()
	d := ds.TestResourceData()
	_ = d.Set("team_id", 99)
	_ = d.Set("user_id", 99)

	if err := rcRead(ds, d, mock.Client()); err == nil {
		t.Fatal("expected error when membership not found, got nil")
	}
}

// TestDataSourceTeamMembershipRead_HTTPError verifies that an HTTP error is
// propagated.
func TestDataSourceTeamMembershipRead_HTTPError(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/team_memberships", RespondString(
		http.StatusInternalServerError, "application/json",
		`{"message":"boom"}`,
	))

	ds := dataSourceTeamMembership()
	d := ds.TestResourceData()
	_ = d.Set("team_id", 1)
	_ = d.Set("user_id", 1)

	if err := rcRead(ds, d, mock.Client()); err == nil {
		t.Fatal("expected error on HTTP 500, got nil")
	}
}
