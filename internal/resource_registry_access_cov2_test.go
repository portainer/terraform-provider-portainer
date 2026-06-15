package internal

import (
	"net/http"
	"testing"
)

// =========================================================================
// Additional coverage (cov2) for resource_registry_access.go: the Update
// delegation, the user-variant Create, the endpoint-not-in-RegistryAccesses
// branch of getRegistryPolicies, the EndpointRegistryAccess SDK error path,
// the Delete user variant, and the Read no-team/no-user no-op.
// =========================================================================

// TestRegistryAccessCov2_Update_DelegatesToCreate verifies Update re-runs
// Create: inspect the registry, PUT the merged policy, then re-read.
func TestRegistryAccessCov2_Update_DelegatesToCreate(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/registries/1", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id":   1,
		"Name": "dockerhub",
		"RegistryAccesses": map[string]interface{}{
			"2": map[string]interface{}{
				"UserAccessPolicies": map[string]interface{}{},
				"TeamAccessPolicies": map[string]interface{}{
					"7": map[string]interface{}{"RoleID": 1},
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
	_ = d.Set("role_id", 1)

	if err := rcUpdate(r, d, mock.Client()); err != nil {
		t.Fatalf("Update failed: %v", err)
	}
	if mock.FindRequest("PUT", "/endpoints/2/registries/1") == nil {
		t.Error("expected PUT /endpoints/2/registries/1 from Update delegation")
	}
}

// TestRegistryAccessCov2_Create_User_NoExistingAccess covers the user variant
// and the branch where the endpoint id is absent from RegistryAccesses (so
// getRegistryPolicies returns freshly-initialised empty policy maps).
func TestRegistryAccessCov2_Create_User_NoExistingAccess(t *testing.T) {
	mock := NewMockServer(t)

	// Stateful: first GET (Create's getRegistryPolicies) sees endpoint "2"
	// absent, so Create initialises fresh empty maps. The chained Read re-GETs
	// and must now see the just-created user access, otherwise it clears the ID.
	getCalls := 0
	mock.On("GET", "/registries/1", func(w http.ResponseWriter, req *http.Request) {
		getCalls++
		accesses := map[string]interface{}{}
		if getCalls > 1 {
			accesses["2"] = map[string]interface{}{
				"UserAccessPolicies": map[string]interface{}{
					"42": map[string]interface{}{"RoleId": 2},
				},
			}
		}
		RespondJSON(http.StatusOK, map[string]interface{}{
			"Id":               1,
			"Name":             "dockerhub",
			"RegistryAccesses": accesses,
		})(w, req)
	})
	mock.On("PUT", "/endpoints/2/registries/1", RespondString(http.StatusNoContent, "", ""))

	r := resourceRegistryAccess()
	d := r.TestResourceData()
	_ = d.Set("registry_id", 1)
	_ = d.Set("endpoint_id", 2)
	_ = d.Set("user_id", 42)
	_ = d.Set("role_id", 2)

	if err := rcCreate(r, d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if d.Id() != "1/2/user/42" {
		t.Errorf("expected composite ID %q, got %q", "1/2/user/42", d.Id())
	}
	put := mock.FindRequest("PUT", "/endpoints/2/registries/1")
	if put == nil {
		t.Fatal("expected PUT /endpoints/2/registries/1")
	}
	var payload map[string]interface{}
	if err := put.DecodeJSON(&payload); err != nil {
		t.Fatalf("decode PUT body: %v", err)
	}
	userPols, ok := payload["userAccessPolicies"].(map[string]interface{})
	if !ok || userPols["42"] == nil {
		t.Errorf("expected user 42 in userAccessPolicies, got %v", payload["userAccessPolicies"])
	}
}

// TestRegistryAccessCov2_Create_PUTError covers the EndpointRegistryAccess SDK
// error path: the inspect succeeds but the access PUT returns a non-2xx.
func TestRegistryAccessCov2_Create_PUTError(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/registries/1", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id":               1,
		"Name":             "dockerhub",
		"RegistryAccesses": map[string]interface{}{},
	}))
	mock.On("PUT", "/endpoints/2/registries/1", RespondString(
		http.StatusInternalServerError, "application/json", `{"message":"boom"}`,
	))

	r := resourceRegistryAccess()
	d := r.TestResourceData()
	_ = d.Set("registry_id", 1)
	_ = d.Set("endpoint_id", 2)
	_ = d.Set("team_id", 7)

	if err := rcCreate(r, d, mock.Client()); err == nil {
		t.Fatal("expected error when registry-access PUT returns 500, got nil")
	}
}

// TestRegistryAccessCov2_Delete_User_HappyPath covers the hasUser delete
// branch.
func TestRegistryAccessCov2_Delete_User_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/registries/1", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id":   1,
		"Name": "dockerhub",
		"RegistryAccesses": map[string]interface{}{
			"2": map[string]interface{}{
				"UserAccessPolicies": map[string]interface{}{
					"42": map[string]interface{}{"RoleID": 2},
				},
				"TeamAccessPolicies": map[string]interface{}{},
			},
		},
	}))
	mock.On("PUT", "/endpoints/2/registries/1", RespondString(http.StatusNoContent, "", ""))

	r := resourceRegistryAccess()
	d := r.TestResourceData()
	d.SetId("1/2/user/42")
	_ = d.Set("registry_id", 1)
	_ = d.Set("endpoint_id", 2)
	_ = d.Set("user_id", 42)

	if err := rcDelete(r, d, mock.Client()); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	if mock.FindRequest("PUT", "/endpoints/2/registries/1") == nil {
		t.Error("expected PUT /endpoints/2/registries/1 for user delete")
	}
}

// TestRegistryAccessCov2_Delete_RegistryGone_NoError covers the
// ErrRegistryNotFound branch of Delete (inspect returns 404 -> success).
func TestRegistryAccessCov2_Delete_RegistryGone_NoError(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/registries/1", RespondString(
		http.StatusNotFound, "application/json", `{"message":"registry not found"}`,
	))

	r := resourceRegistryAccess()
	d := r.TestResourceData()
	d.SetId("1/2/team/7")
	_ = d.Set("registry_id", 1)
	_ = d.Set("endpoint_id", 2)
	_ = d.Set("team_id", 7)

	if err := rcDelete(r, d, mock.Client()); err != nil {
		t.Fatalf("Delete should swallow registry-not-found, got error: %v", err)
	}
}

// TestRegistryAccessCov2_Read_NoTeamOrUser covers the Read no-op when neither
// team_id nor user_id is set: found stays false and the ID is cleared.
func TestRegistryAccessCov2_Read_NoTeamOrUser(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/registries/1", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id":               1,
		"Name":             "dockerhub",
		"RegistryAccesses": map[string]interface{}{},
	}))

	r := resourceRegistryAccess()
	d := r.TestResourceData()
	d.SetId("1/2/")
	_ = d.Set("registry_id", 1)
	_ = d.Set("endpoint_id", 2)
	// neither team_id nor user_id set

	if err := rcRead(r, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if d.Id() != "" {
		t.Errorf("expected ID cleared when no team/user policy, got %q", d.Id())
	}
}
