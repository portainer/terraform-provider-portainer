package internal

import (
	"net/http"
	"testing"
)

// =========================================================================
// Additional coverage (cov3) for resource_endpoint_group_access.go: the
// non-404 fetch-error branch reached from Read, the PUT-returns-404 swallow in
// Delete, and a Create that sets both team_id and user_id (exercising both
// policy-merge branches in a single call).
// =========================================================================

// TestEndpointGroupAccessCov3_Read_HTTPError covers the generic (non-404)
// error branch of Read when getEndpointGroupPolicies fails.
func TestEndpointGroupAccessCov3_Read_HTTPError(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/endpoint_groups/4", RespondString(
		http.StatusInternalServerError, "application/json", `{"message":"boom"}`,
	))

	r := resourceEndpointGroupAccess()
	d := r.TestResourceData()
	d.SetId("4/team/11")
	_ = d.Set("endpoint_group_id", 4)
	_ = d.Set("team_id", 11)

	if err := rcRead(r, d, mock.Client()); err == nil {
		t.Fatal("expected error on HTTP 500 during Read, got nil")
	}
}

// TestEndpointGroupAccessCov3_Delete_PUT404_NoError covers the branch where the
// group fetch succeeds but the write-back PUT returns 404 — treated as success
// (the group is already gone).
func TestEndpointGroupAccessCov3_Delete_PUT404_NoError(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/endpoint_groups/4", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id": 4,
		"TeamAccessPolicies": map[string]interface{}{
			"11": map[string]interface{}{"RoleId": 3},
		},
		"UserAccessPolicies": map[string]interface{}{},
	}))
	mock.On("PUT", "/endpoint_groups/4", RespondString(
		http.StatusNotFound, "application/json", `{"message":"gone"}`,
	))

	r := resourceEndpointGroupAccess()
	d := r.TestResourceData()
	d.SetId("4/team/11")
	_ = d.Set("endpoint_group_id", 4)
	_ = d.Set("team_id", 11)

	if err := rcDelete(r, d, mock.Client()); err != nil {
		t.Fatalf("Delete should swallow PUT 404, got error: %v", err)
	}
}

// TestEndpointGroupAccessCov3_Create_TeamAndUser sets both team_id and user_id
// so Create executes both the hasTeam and hasUser merge branches in the same
// call. The final Read returns the team policy so the ID (team variant) stays.
func TestEndpointGroupAccessCov3_Create_TeamAndUser(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/endpoint_groups/7", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id":   7,
		"Name": "g7",
		"TeamAccessPolicies": map[string]interface{}{
			"11": map[string]interface{}{"RoleId": 2},
		},
		"UserAccessPolicies": map[string]interface{}{
			"5": map[string]interface{}{"RoleId": 2},
		},
	}))
	mock.On("PUT", "/endpoint_groups/7", RespondJSON(http.StatusOK, map[string]interface{}{"Id": 7}))

	r := resourceEndpointGroupAccess()
	d := r.TestResourceData()
	_ = d.Set("endpoint_group_id", 7)
	_ = d.Set("team_id", 11)
	_ = d.Set("user_id", 5)
	_ = d.Set("role_id", 2)

	if err := rcCreate(r, d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	// hasTeam wins the ID-building branch.
	if d.Id() != "7/team/11" {
		t.Errorf("expected ID %q, got %q", "7/team/11", d.Id())
	}

	put := mock.FindRequest("PUT", "/endpoint_groups/7")
	if put == nil {
		t.Fatal("expected PUT /endpoint_groups/7")
	}
	var payload map[string]interface{}
	if err := put.DecodeJSON(&payload); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	teams, _ := payload["TeamAccessPolicies"].(map[string]interface{})
	if teams == nil || teams["11"] == nil {
		t.Errorf("expected team 11 in TeamAccessPolicies, got %v", teams)
	}
	users, _ := payload["UserAccessPolicies"].(map[string]interface{})
	if users == nil || users["5"] == nil {
		t.Errorf("expected user 5 in UserAccessPolicies, got %v", users)
	}
}
