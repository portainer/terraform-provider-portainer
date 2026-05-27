package internal

import (
	"net/http"
	"testing"
)

// TestEndpointGroupAccessCreate_Team_HappyPath verifies that creating a
// team access entry: (1) GETs the existing endpoint-group object,
// (2) PUTs it back with the new team policy merged in, (3) populates the
// composite ID "<group>/team/<teamID>" and re-reads role_id.
func TestEndpointGroupAccessCreate_Team_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	// The Create path issues two GETs (legacy getEndpointGroupPolicies then
	// getEndpointGroupMap) before PUT, and then chains into Read which GETs
	// again to verify the policy landed. Return the policy already in place
	// so the final Read does not clear the ID.
	mock.On("GET", "/endpoint_groups/4", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id":                 4,
		"Name":               "g4",
		"UserAccessPolicies": map[string]interface{}{},
		"TeamAccessPolicies": map[string]interface{}{
			"11": map[string]interface{}{"RoleId": 3},
		},
	}))

	mock.On("PUT", "/endpoint_groups/4", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id":   4,
		"Name": "g4",
		"TeamAccessPolicies": map[string]interface{}{
			"11": map[string]interface{}{"RoleId": 3},
		},
	}))

	r := resourceEndpointGroupAccess()
	d := r.TestResourceData()
	_ = d.Set("endpoint_group_id", 4)
	_ = d.Set("team_id", 11)
	_ = d.Set("role_id", 3)

	if err := r.Create(d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	if d.Id() != "4/team/11" {
		t.Errorf("expected ID %q, got %q", "4/team/11", d.Id())
	}

	put := mock.FindRequest("PUT", "/endpoint_groups/4")
	if put == nil {
		t.Fatal("expected PUT /endpoint_groups/4")
	}
	var payload map[string]interface{}
	if err := put.DecodeJSON(&payload); err != nil {
		t.Fatalf("failed to decode PUT body: %v", err)
	}
	teams, ok := payload["TeamAccessPolicies"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected TeamAccessPolicies in payload, got %v", payload["TeamAccessPolicies"])
	}
	entry, ok := teams["11"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected team key 11 in payload, got %v", teams)
	}
	if entry["RoleId"] != float64(3) {
		t.Errorf("team[11].RoleId: expected 3, got %v", entry["RoleId"])
	}
}

// TestEndpointGroupAccessCreate_User_HappyPath verifies the user variant.
func TestEndpointGroupAccessCreate_User_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/endpoint_groups/2", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id":   2,
		"Name": "g2",
		"UserAccessPolicies": map[string]interface{}{
			"5": map[string]interface{}{"RoleId": 1},
		},
		"TeamAccessPolicies": map[string]interface{}{},
	}))
	mock.On("PUT", "/endpoint_groups/2", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id": 2,
	}))

	r := resourceEndpointGroupAccess()
	d := r.TestResourceData()
	_ = d.Set("endpoint_group_id", 2)
	_ = d.Set("user_id", 5)
	_ = d.Set("role_id", 1)

	if err := r.Create(d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if d.Id() != "2/user/5" {
		t.Errorf("expected ID %q, got %q", "2/user/5", d.Id())
	}

	put := mock.FindRequest("PUT", "/endpoint_groups/2")
	if put == nil {
		t.Fatal("expected PUT /endpoint_groups/2")
	}
	var payload map[string]interface{}
	_ = put.DecodeJSON(&payload)
	users, _ := payload["UserAccessPolicies"].(map[string]interface{})
	if users == nil || users["5"] == nil {
		t.Errorf("expected user 5 in UserAccessPolicies, got %v", users)
	}
}

// TestEndpointGroupAccessCreate_MissingTeamAndUser verifies that omitting
// both team_id and user_id is rejected.
func TestEndpointGroupAccessCreate_MissingTeamAndUser(t *testing.T) {
	mock := NewMockServer(t)

	r := resourceEndpointGroupAccess()
	d := r.TestResourceData()
	_ = d.Set("endpoint_group_id", 1)

	if err := r.Create(d, mock.Client()); err == nil {
		t.Fatal("expected error when neither team_id nor user_id is set")
	}
}

// TestEndpointGroupAccessRead_Team_HappyPath verifies that the role_id is
// read out of TeamAccessPolicies for the configured team.
func TestEndpointGroupAccessRead_Team_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/endpoint_groups/4", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id": 4,
		"TeamAccessPolicies": map[string]interface{}{
			"11": map[string]interface{}{"RoleId": 3},
		},
		"UserAccessPolicies": map[string]interface{}{},
	}))

	r := resourceEndpointGroupAccess()
	d := r.TestResourceData()
	d.SetId("4/team/11")
	_ = d.Set("endpoint_group_id", 4)
	_ = d.Set("team_id", 11)

	if err := r.Read(d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if got := d.Get("role_id"); got != 3 {
		t.Errorf("role_id: expected 3, got %v", got)
	}
	if d.Id() == "" {
		t.Errorf("expected ID to remain set, got cleared")
	}
}

// TestEndpointGroupAccessRead_404_ClearsID verifies that a 404 on the GET
// removes the resource from state.
func TestEndpointGroupAccessRead_404_ClearsID(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/endpoint_groups/99", RespondString(
		http.StatusNotFound, "application/json",
		`{"message":"not found"}`,
	))

	r := resourceEndpointGroupAccess()
	d := r.TestResourceData()
	d.SetId("99/team/1")
	_ = d.Set("endpoint_group_id", 99)
	_ = d.Set("team_id", 1)

	if err := r.Read(d, mock.Client()); err != nil {
		t.Fatalf("Read should swallow 404 and clear ID, got error: %v", err)
	}
	if d.Id() != "" {
		t.Errorf("expected ID cleared after 404, got %q", d.Id())
	}
}

// TestEndpointGroupAccessRead_NotPresentClearsID verifies that if the GET
// succeeds but the policy for the team/user is gone (drift), the ID is
// cleared.
func TestEndpointGroupAccessRead_NotPresentClearsID(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/endpoint_groups/4", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id":                 4,
		"TeamAccessPolicies": map[string]interface{}{},
		"UserAccessPolicies": map[string]interface{}{},
	}))

	r := resourceEndpointGroupAccess()
	d := r.TestResourceData()
	d.SetId("4/team/11")
	_ = d.Set("endpoint_group_id", 4)
	_ = d.Set("team_id", 11)

	if err := r.Read(d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if d.Id() != "" {
		t.Errorf("expected ID cleared when policy missing, got %q", d.Id())
	}
}

// TestEndpointGroupAccessDelete_HappyPath verifies that the policy is
// removed from the map and PUT back.
func TestEndpointGroupAccessDelete_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/endpoint_groups/4", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id": 4,
		"TeamAccessPolicies": map[string]interface{}{
			"11": map[string]interface{}{"RoleId": 3},
		},
		"UserAccessPolicies": map[string]interface{}{},
	}))
	mock.On("PUT", "/endpoint_groups/4", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id": 4,
	}))

	r := resourceEndpointGroupAccess()
	d := r.TestResourceData()
	d.SetId("4/team/11")
	_ = d.Set("endpoint_group_id", 4)
	_ = d.Set("team_id", 11)

	if err := r.Delete(d, mock.Client()); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	put := mock.FindRequest("PUT", "/endpoint_groups/4")
	if put == nil {
		t.Fatal("expected PUT /endpoint_groups/4 (delete-via-update)")
	}
	var payload map[string]interface{}
	_ = put.DecodeJSON(&payload)
	teams, _ := payload["TeamAccessPolicies"].(map[string]interface{})
	if _, exists := teams["11"]; exists {
		t.Errorf("expected team 11 to be removed from TeamAccessPolicies, got %v", teams)
	}
}

// TestEndpointGroupAccessCreate_HTTPError verifies that a 4xx on the GET
// fetch surfaces as an error.
func TestEndpointGroupAccessCreate_HTTPError(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/endpoint_groups/4", RespondString(
		http.StatusInternalServerError, "application/json",
		`{"message":"boom"}`,
	))

	r := resourceEndpointGroupAccess()
	d := r.TestResourceData()
	_ = d.Set("endpoint_group_id", 4)
	_ = d.Set("team_id", 11)

	if err := r.Create(d, mock.Client()); err == nil {
		t.Fatal("expected error on HTTP 500, got nil")
	}
}
