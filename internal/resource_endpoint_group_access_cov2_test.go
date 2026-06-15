package internal

import (
	"net/http"
	"testing"
)

// =========================================================================
// Additional coverage (cov2) for resource_endpoint_group_access.go: the
// Update delegation, the user-variant Read, the Delete user variant plus its
// 404-swallow and fetch-error branches, and the updateEndpointGroup PUT-error
// path.
// =========================================================================

// TestEndpointGroupAccessCov2_Update_DelegatesToCreate verifies Update simply
// re-runs Create: GET the group, PUT it back with the merged policy, then
// re-read. Mirrors the Create happy path via the Update entry point.
func TestEndpointGroupAccessCov2_Update_DelegatesToCreate(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/endpoint_groups/4", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id":                 4,
		"Name":               "g4",
		"UserAccessPolicies": map[string]interface{}{},
		"TeamAccessPolicies": map[string]interface{}{
			"11": map[string]interface{}{"RoleId": 2},
		},
	}))
	mock.On("PUT", "/endpoint_groups/4", RespondJSON(http.StatusOK, map[string]interface{}{"Id": 4}))

	r := resourceEndpointGroupAccess()
	d := r.TestResourceData()
	d.SetId("4/team/11")
	_ = d.Set("endpoint_group_id", 4)
	_ = d.Set("team_id", 11)
	_ = d.Set("role_id", 2)

	if err := rcUpdate(r, d, mock.Client()); err != nil {
		t.Fatalf("Update failed: %v", err)
	}
	if mock.FindRequest("PUT", "/endpoint_groups/4") == nil {
		t.Error("expected PUT /endpoint_groups/4 from Update delegation")
	}
}

// TestEndpointGroupAccessCov2_Read_User_HappyPath covers the hasUser branch of
// Read (the team-variant is exercised elsewhere).
func TestEndpointGroupAccessCov2_Read_User_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/endpoint_groups/2", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id":                 2,
		"TeamAccessPolicies": map[string]interface{}{},
		"UserAccessPolicies": map[string]interface{}{
			"5": map[string]interface{}{"RoleId": 4},
		},
	}))

	r := resourceEndpointGroupAccess()
	d := r.TestResourceData()
	d.SetId("2/user/5")
	_ = d.Set("endpoint_group_id", 2)
	_ = d.Set("user_id", 5)

	if err := rcRead(r, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if got := d.Get("role_id"); got != 4 {
		t.Errorf("role_id: expected 4, got %v", got)
	}
	if d.Id() == "" {
		t.Error("expected ID to remain set for user policy")
	}
}

// TestEndpointGroupAccessCov2_Delete_User_HappyPath covers the hasUser delete
// branch removing the user policy.
func TestEndpointGroupAccessCov2_Delete_User_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/endpoint_groups/2", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id":                 2,
		"TeamAccessPolicies": map[string]interface{}{},
		"UserAccessPolicies": map[string]interface{}{
			"5": map[string]interface{}{"RoleId": 4},
		},
	}))
	mock.On("PUT", "/endpoint_groups/2", RespondJSON(http.StatusOK, map[string]interface{}{"Id": 2}))

	r := resourceEndpointGroupAccess()
	d := r.TestResourceData()
	d.SetId("2/user/5")
	_ = d.Set("endpoint_group_id", 2)
	_ = d.Set("user_id", 5)

	if err := rcDelete(r, d, mock.Client()); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	put := mock.FindRequest("PUT", "/endpoint_groups/2")
	if put == nil {
		t.Fatal("expected PUT /endpoint_groups/2 for user delete")
	}
	var payload map[string]interface{}
	_ = put.DecodeJSON(&payload)
	users, _ := payload["UserAccessPolicies"].(map[string]interface{})
	if _, exists := users["5"]; exists {
		t.Errorf("expected user 5 removed from UserAccessPolicies, got %v", users)
	}
}

// TestEndpointGroupAccessCov2_Delete_GroupGone_NoError covers the
// ErrEndpointGroupNotFound branch in Delete: a 404 on the group fetch is
// swallowed (the group is already gone).
func TestEndpointGroupAccessCov2_Delete_GroupGone_NoError(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/endpoint_groups/99", RespondString(
		http.StatusNotFound, "application/json", `{"message":"not found"}`,
	))

	r := resourceEndpointGroupAccess()
	d := r.TestResourceData()
	d.SetId("99/team/1")
	_ = d.Set("endpoint_group_id", 99)
	_ = d.Set("team_id", 1)

	if err := rcDelete(r, d, mock.Client()); err != nil {
		t.Fatalf("Delete should swallow group-not-found, got error: %v", err)
	}
}

// TestEndpointGroupAccessCov2_Delete_FetchError covers the generic (non-404)
// fetch-error branch in Delete.
func TestEndpointGroupAccessCov2_Delete_FetchError(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/endpoint_groups/4", RespondString(
		http.StatusInternalServerError, "application/json", `{"message":"boom"}`,
	))

	r := resourceEndpointGroupAccess()
	d := r.TestResourceData()
	d.SetId("4/team/11")
	_ = d.Set("endpoint_group_id", 4)
	_ = d.Set("team_id", 11)

	if err := rcDelete(r, d, mock.Client()); err == nil {
		t.Fatal("expected error on HTTP 500 group fetch during Delete, got nil")
	}
}

// TestEndpointGroupAccessCov2_Delete_PUTError covers the >= 400 branch of the
// Delete PUT: the group fetch succeeds but writing it back fails.
func TestEndpointGroupAccessCov2_Delete_PUTError(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/endpoint_groups/4", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id": 4,
		"TeamAccessPolicies": map[string]interface{}{
			"11": map[string]interface{}{"RoleId": 3},
		},
		"UserAccessPolicies": map[string]interface{}{},
	}))
	mock.On("PUT", "/endpoint_groups/4", RespondString(
		http.StatusInternalServerError, "application/json", `{"message":"put boom"}`,
	))

	r := resourceEndpointGroupAccess()
	d := r.TestResourceData()
	d.SetId("4/team/11")
	_ = d.Set("endpoint_group_id", 4)
	_ = d.Set("team_id", 11)

	if err := rcDelete(r, d, mock.Client()); err == nil {
		t.Fatal("expected error on HTTP 500 Delete PUT, got nil")
	}
}

// TestEndpointGroupAccessCov2_Create_PUTError covers the updateEndpointGroup
// PUT-error path reached from Create: both GETs succeed but the PUT fails.
func TestEndpointGroupAccessCov2_Create_PUTError(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/endpoint_groups/4", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id":                 4,
		"Name":               "g4",
		"UserAccessPolicies": map[string]interface{}{},
		"TeamAccessPolicies": map[string]interface{}{},
	}))
	mock.On("PUT", "/endpoint_groups/4", RespondString(
		http.StatusBadRequest, "application/json", `{"message":"put boom"}`,
	))

	r := resourceEndpointGroupAccess()
	d := r.TestResourceData()
	_ = d.Set("endpoint_group_id", 4)
	_ = d.Set("team_id", 11)
	_ = d.Set("role_id", 3)

	if err := rcCreate(r, d, mock.Client()); err == nil {
		t.Fatal("expected error on HTTP 400 Create PUT, got nil")
	}
}
