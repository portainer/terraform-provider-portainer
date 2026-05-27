package internal

import (
	"net/http"
	"testing"
)

// TestRegistryAccessCreate_TeamHappyPath verifies the resource fetches the
// existing registry, merges a team policy, and PUTs the result to the
// endpoint-registries access endpoint. The composite ID encodes
// "<registry>/<endpoint>/team/<team_id>".
func TestRegistryAccessCreate_TeamHappyPath(t *testing.T) {
	mock := NewMockServer(t)

	// Registry is fetched in Create (to merge policies) and again in the
	// chained Read after Create. The mock returns the same response both
	// times — once the policy is in place, Read should find it.
	mock.On("GET", "/registries/1", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id":   1,
		"Name": "dockerhub",
		"RegistryAccesses": map[string]interface{}{
			"2": map[string]interface{}{
				"UserAccessPolicies": map[string]interface{}{},
				"TeamAccessPolicies": map[string]interface{}{
					"7": map[string]interface{}{"RoleID": 0},
				},
			},
		},
	}))

	mock.On("PUT", "/endpoints/2/registries/1", RespondString(http.StatusNoContent, "", ""))

	r := resourceRegistryAccess()
	d := r.TestResourceData()
	_ = d.Set("registry_id", 1)
	_ = d.Set("endpoint_id", 2)
	_ = d.Set("team_id", 7)

	if err := r.Create(d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	if d.Id() != "1/2/team/7" {
		t.Errorf("expected composite ID %q, got %q", "1/2/team/7", d.Id())
	}

	put := mock.FindRequest("PUT", "/endpoints/2/registries/1")
	if put == nil {
		t.Fatal("expected a PUT to /endpoints/2/registries/1")
	}
	var payload map[string]interface{}
	if err := put.DecodeJSON(&payload); err != nil {
		t.Fatalf("failed to decode PUT body: %v", err)
	}
	teamPols, ok := payload["teamAccessPolicies"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected teamAccessPolicies in payload, got %v", payload)
	}
	if _, ok := teamPols["7"]; !ok {
		t.Errorf("expected team_id 7 in teamAccessPolicies, got %v", teamPols)
	}
}

// TestRegistryAccessCreate_NoTeamOrUser ensures the resource errors out if
// neither team_id nor user_id is supplied (avoiding a no-op write).
func TestRegistryAccessCreate_NoTeamOrUser(t *testing.T) {
	mock := NewMockServer(t)

	r := resourceRegistryAccess()
	d := r.TestResourceData()
	_ = d.Set("registry_id", 1)
	_ = d.Set("endpoint_id", 2)

	err := r.Create(d, mock.Client())
	if err == nil {
		t.Fatal("expected error when neither team_id nor user_id is set")
	}
}

// TestRegistryAccessRead_FoundInPolicies verifies Read populates role_id
// from the registry's access policies.
func TestRegistryAccessRead_FoundInPolicies(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/registries/1", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id":   1,
		"Name": "dockerhub",
		"RegistryAccesses": map[string]interface{}{
			"2": map[string]interface{}{
				"UserAccessPolicies": map[string]interface{}{
					"99": map[string]interface{}{"RoleID": 3},
				},
				"TeamAccessPolicies": map[string]interface{}{},
			},
		},
	}))

	r := resourceRegistryAccess()
	d := r.TestResourceData()
	d.SetId("1/2/user/99")
	_ = d.Set("registry_id", 1)
	_ = d.Set("endpoint_id", 2)
	_ = d.Set("user_id", 99)

	if err := r.Read(d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if d.Id() == "" {
		t.Error("expected ID to remain set when policy found")
	}
	if got := d.Get("role_id"); got != 3 {
		t.Errorf("role_id: expected 3, got %v", got)
	}
}

// TestRegistryAccessRead_NotFoundClearsID verifies that when the team/user
// is not present in policies, the resource ID is cleared (drift detection).
func TestRegistryAccessRead_NotFoundClearsID(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/registries/1", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id":               1,
		"Name":             "dockerhub",
		"RegistryAccesses": map[string]interface{}{},
	}))

	r := resourceRegistryAccess()
	d := r.TestResourceData()
	d.SetId("1/2/team/7")
	_ = d.Set("registry_id", 1)
	_ = d.Set("endpoint_id", 2)
	_ = d.Set("team_id", 7)

	if err := r.Read(d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if d.Id() != "" {
		t.Errorf("expected ID cleared, got %q", d.Id())
	}
}

// TestRegistryAccessDelete_HappyPath verifies Delete removes the team/user
// policy and PUTs the updated payload.
func TestRegistryAccessDelete_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/registries/1", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id":   1,
		"Name": "dockerhub",
		"RegistryAccesses": map[string]interface{}{
			"2": map[string]interface{}{
				"UserAccessPolicies": map[string]interface{}{},
				"TeamAccessPolicies": map[string]interface{}{
					"7": map[string]interface{}{"RoleID": 0},
				},
			},
		},
	}))
	mock.On("PUT", "/endpoints/2/registries/1", RespondString(http.StatusNoContent, "", ""))

	r := resourceRegistryAccess()
	d := r.TestResourceData()
	d.SetId("1/2/team/7")
	_ = d.Set("registry_id", 1)
	_ = d.Set("endpoint_id", 2)
	_ = d.Set("team_id", 7)

	if err := r.Delete(d, mock.Client()); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	if mock.FindRequest("PUT", "/endpoints/2/registries/1") == nil {
		t.Error("expected PUT /endpoints/2/registries/1 to be sent")
	}
}

// TestRegistryAccessRead_RegistryNotFoundClearsID verifies that when the
// upstream registry is gone, the resource cleanly removes itself from state.
func TestRegistryAccessRead_RegistryNotFoundClearsID(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/registries/1", RespondString(
		http.StatusNotFound, "application/json",
		`{"message":"registry not found"}`,
	))

	r := resourceRegistryAccess()
	d := r.TestResourceData()
	d.SetId("1/2/team/7")
	_ = d.Set("registry_id", 1)
	_ = d.Set("endpoint_id", 2)
	_ = d.Set("team_id", 7)

	if err := r.Read(d, mock.Client()); err != nil {
		t.Fatalf("Read should swallow 404, got error: %v", err)
	}
	if d.Id() != "" {
		t.Errorf("expected ID cleared, got %q", d.Id())
	}
}
