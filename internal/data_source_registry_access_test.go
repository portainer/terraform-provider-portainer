package internal

import (
	"net/http"
	"testing"
)

// TestDataSourceRegistryAccessRead_TeamHappyPath verifies the data source
// pulls the team's role ID out of the registry's RegistryAccesses map and
// builds the composite Terraform ID.
func TestDataSourceRegistryAccessRead_TeamHappyPath(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/registries/4", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id":   4,
		"Name": "harbor",
		"RegistryAccesses": map[string]interface{}{
			"2": map[string]interface{}{
				"UserAccessPolicies": map[string]interface{}{},
				"TeamAccessPolicies": map[string]interface{}{
					"9": map[string]interface{}{"RoleId": 3},
				},
				"Namespaces": []string{},
			},
		},
	}))

	ds := dataSourceRegistryAccess()
	d := ds.TestResourceData()
	_ = d.Set("registry_id", 4)
	_ = d.Set("endpoint_id", 2)
	_ = d.Set("team_id", 9)

	if err := ds.Read(d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	if d.Id() != "4/2/team/9" {
		t.Errorf("expected composite ID %q, got %q", "4/2/team/9", d.Id())
	}
	if got := d.Get("role_id"); got != 3 {
		t.Errorf("role_id: expected 3, got %v", got)
	}
}

// TestDataSourceRegistryAccessRead_UserHappyPath verifies the user_id branch.
func TestDataSourceRegistryAccessRead_UserHappyPath(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/registries/4", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id":   4,
		"Name": "harbor",
		"RegistryAccesses": map[string]interface{}{
			"2": map[string]interface{}{
				"UserAccessPolicies": map[string]interface{}{
					"7": map[string]interface{}{"RoleId": 4},
				},
				"TeamAccessPolicies": map[string]interface{}{},
				"Namespaces":         []string{},
			},
		},
	}))

	ds := dataSourceRegistryAccess()
	d := ds.TestResourceData()
	_ = d.Set("registry_id", 4)
	_ = d.Set("endpoint_id", 2)
	_ = d.Set("user_id", 7)

	if err := ds.Read(d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	if d.Id() != "4/2/user/7" {
		t.Errorf("expected composite ID %q, got %q", "4/2/user/7", d.Id())
	}
	if got := d.Get("role_id"); got != 4 {
		t.Errorf("role_id: expected 4, got %v", got)
	}
}

// TestDataSourceRegistryAccessRead_PolicyMissing errors out when neither the
// team nor the user has an access policy on the registry.
func TestDataSourceRegistryAccessRead_PolicyMissing(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/registries/4", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id":               4,
		"Name":             "harbor",
		"RegistryAccesses": map[string]interface{}{},
	}))

	ds := dataSourceRegistryAccess()
	d := ds.TestResourceData()
	_ = d.Set("registry_id", 4)
	_ = d.Set("endpoint_id", 2)
	_ = d.Set("team_id", 9)

	if err := ds.Read(d, mock.Client()); err == nil {
		t.Fatal("expected error when access policy is missing, got nil")
	}
}

// TestDataSourceRegistryAccessRead_RequiresTeamOrUser fails fast if neither
// team_id nor user_id is provided.
func TestDataSourceRegistryAccessRead_RequiresTeamOrUser(t *testing.T) {
	mock := NewMockServer(t)

	ds := dataSourceRegistryAccess()
	d := ds.TestResourceData()
	_ = d.Set("registry_id", 4)
	_ = d.Set("endpoint_id", 2)

	if err := ds.Read(d, mock.Client()); err == nil {
		t.Fatal("expected error when neither team_id nor user_id is set, got nil")
	}
}
