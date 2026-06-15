package internal

import (
	"context"
	"net/http"
	"testing"
)

// =========================================================================
// Additional coverage (cov2) for resource_team_membership.go: the SDK error
// branches on Create / Read-list / Update / Delete, and the Importer func
// (which delegates to Read).
// =========================================================================

// TestTeamMembershipCov2_Import_HappyPath exercises resourceTeamMembershipImport
// which delegates to Read: a list containing the ID hydrates state and returns
// the resource data.
func TestTeamMembershipCov2_Import_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/team_memberships", RespondJSON(http.StatusOK, []map[string]interface{}{
		{"Id": 11, "Role": 2, "TeamID": 5, "UserID": 7},
	}))

	r := resourceTeamMembership()
	d := r.TestResourceData()
	d.SetId("11")

	out, err := r.Importer.StateContext(context.Background(), d, mock.Client())
	if err != nil {
		t.Fatalf("import failed: %v", err)
	}
	if len(out) != 1 {
		t.Fatalf("expected 1 resource data, got %d", len(out))
	}
	if got := out[0].Get("team_id"); got != 5 {
		t.Errorf("team_id: expected 5, got %v", got)
	}
}

// TestTeamMembershipCov2_Import_ListError covers the import error path: the
// underlying Read (list) fails, so import surfaces an error.
func TestTeamMembershipCov2_Import_ListError(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/team_memberships", RespondString(
		http.StatusInternalServerError, "application/json", `{"message":"boom"}`,
	))

	r := resourceTeamMembership()
	d := r.TestResourceData()
	d.SetId("11")

	if _, err := r.Importer.StateContext(context.Background(), d, mock.Client()); err == nil {
		t.Fatal("expected error when membership list fails during import, got nil")
	}
}

// TestTeamMembershipCov2_Read_ListError covers the Read list-error branch.
func TestTeamMembershipCov2_Read_ListError(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/team_memberships", RespondString(
		http.StatusInternalServerError, "application/json", `{"message":"boom"}`,
	))

	r := resourceTeamMembership()
	d := r.TestResourceData()
	d.SetId("11")

	if err := rcRead(r, d, mock.Client()); err == nil {
		t.Fatal("expected error when membership list returns 500, got nil")
	}
}

// TestTeamMembershipCov2_Create_HTTPError covers the create error branch.
func TestTeamMembershipCov2_Create_HTTPError(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("POST", "/team_memberships", RespondString(
		http.StatusBadRequest, "application/json", `{"message":"invalid"}`,
	))

	r := resourceTeamMembership()
	d := r.TestResourceData()
	_ = d.Set("role", 2)
	_ = d.Set("team_id", 5)
	_ = d.Set("user_id", 7)

	if err := rcCreate(r, d, mock.Client()); err == nil {
		t.Fatal("expected error on create 400, got nil")
	}
}

// TestTeamMembershipCov2_Update_HTTPError covers the update error branch.
func TestTeamMembershipCov2_Update_HTTPError(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("PUT", "/team_memberships/3", RespondString(
		http.StatusInternalServerError, "application/json", `{"message":"boom"}`,
	))

	r := resourceTeamMembership()
	d := r.TestResourceData()
	d.SetId("3")
	_ = d.Set("role", 1)
	_ = d.Set("team_id", 8)
	_ = d.Set("user_id", 9)

	if err := rcUpdate(r, d, mock.Client()); err == nil {
		t.Fatal("expected error on update 500, got nil")
	}
}

// TestTeamMembershipCov2_Delete_HTTPError covers the non-404 delete error
// branch (404 is exercised in the base test file).
func TestTeamMembershipCov2_Delete_HTTPError(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("DELETE", "/team_memberships/22", RespondString(
		http.StatusInternalServerError, "application/json", `{"message":"boom"}`,
	))

	r := resourceTeamMembership()
	d := r.TestResourceData()
	d.SetId("22")

	if err := rcDelete(r, d, mock.Client()); err == nil {
		t.Fatal("expected error on delete 500, got nil")
	}
}
