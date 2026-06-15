package internal

import (
	"net/http"
	"testing"
)

// TestDataSourceEndpointGroupAccessRead_Team_HappyPath verifies that the
// role_id is read out of TeamAccessPolicies and the composite ID is built.
func TestDataSourceEndpointGroupAccessRead_Team_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/endpoint_groups/4", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id":                 4,
		"UserAccessPolicies": map[string]interface{}{},
		"TeamAccessPolicies": map[string]interface{}{
			"11": map[string]interface{}{"RoleId": 3},
		},
	}))

	ds := dataSourceEndpointGroupAccess()
	d := ds.TestResourceData()
	_ = d.Set("endpoint_group_id", 4)
	_ = d.Set("team_id", 11)

	if err := rcRead(ds, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	if d.Id() != "4/team/11" {
		t.Errorf("expected ID %q, got %q", "4/team/11", d.Id())
	}
	if got := d.Get("role_id"); got != 3 {
		t.Errorf("role_id: expected 3, got %v", got)
	}
}

// TestDataSourceEndpointGroupAccessRead_User_HappyPath verifies the user
// branch.
func TestDataSourceEndpointGroupAccessRead_User_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/endpoint_groups/2", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id": 2,
		"UserAccessPolicies": map[string]interface{}{
			"5": map[string]interface{}{"RoleId": 1},
		},
		"TeamAccessPolicies": map[string]interface{}{},
	}))

	ds := dataSourceEndpointGroupAccess()
	d := ds.TestResourceData()
	_ = d.Set("endpoint_group_id", 2)
	_ = d.Set("user_id", 5)

	if err := rcRead(ds, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	if d.Id() != "2/user/5" {
		t.Errorf("expected ID %q, got %q", "2/user/5", d.Id())
	}
	if got := d.Get("role_id"); got != 1 {
		t.Errorf("role_id: expected 1, got %v", got)
	}
}

// TestDataSourceEndpointGroupAccessRead_MissingTeamAndUser verifies that
// omitting both team_id and user_id is rejected before any HTTP call.
func TestDataSourceEndpointGroupAccessRead_MissingTeamAndUser(t *testing.T) {
	mock := NewMockServer(t)

	ds := dataSourceEndpointGroupAccess()
	d := ds.TestResourceData()
	_ = d.Set("endpoint_group_id", 4)

	if err := rcRead(ds, d, mock.Client()); err == nil {
		t.Fatal("expected error when both team_id and user_id are unset, got nil")
	}
}

// TestDataSourceEndpointGroupAccessRead_PolicyMissing verifies that when the
// endpoint group exists but the requested team/user is not in its policies,
// an error is returned (DS, not a resource — does not silently clear).
func TestDataSourceEndpointGroupAccessRead_PolicyMissing(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/endpoint_groups/4", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id":                 4,
		"TeamAccessPolicies": map[string]interface{}{},
		"UserAccessPolicies": map[string]interface{}{},
	}))

	ds := dataSourceEndpointGroupAccess()
	d := ds.TestResourceData()
	_ = d.Set("endpoint_group_id", 4)
	_ = d.Set("team_id", 11)

	if err := rcRead(ds, d, mock.Client()); err == nil {
		t.Fatal("expected error when policy missing, got nil")
	}
}

// TestDataSourceEndpointGroupAccessRead_GroupNotFound verifies the 404 path
// surfaces a descriptive error.
func TestDataSourceEndpointGroupAccessRead_GroupNotFound(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/endpoint_groups/99", RespondString(
		http.StatusNotFound, "application/json",
		`{"message":"not found"}`,
	))

	ds := dataSourceEndpointGroupAccess()
	d := ds.TestResourceData()
	_ = d.Set("endpoint_group_id", 99)
	_ = d.Set("team_id", 1)

	if err := rcRead(ds, d, mock.Client()); err == nil {
		t.Fatal("expected error on 404, got nil")
	}
}
