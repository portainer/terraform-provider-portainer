package internal

import (
	"net/http"
	"strings"
	"testing"
)

func TestDataSourceKubernetesNamespaceAccess(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("GET", "/endpoints/3/namespaces/team-a/access", RespondJSON(http.StatusOK, map[string]interface{}{
		"UserAccessPolicies": map[string]interface{}{
			"12": map[string]interface{}{"RoleId": 4, "Namespaces": []string{"team-a"}},
			"2":  map[string]interface{}{"RoleId": 1},
		},
		"TeamAccessPolicies": map[string]interface{}{
			"7": map[string]interface{}{"RoleId": 3},
		},
	}))

	ds := dataSourceKubernetesNamespaceAccess()
	d := ds.TestResourceData()
	_ = d.Set("endpoint_id", 3)
	_ = d.Set("namespace_id", "team-a")
	if err := rcRead(ds, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	// Ordered by ID, numerically.
	for field, want := range map[string]interface{}{
		"user_access.0.user_id": 2, "user_access.0.role_id": 1,
		"user_access.1.user_id": 12, "user_access.1.role_id": 4, "user_access.1.namespaces.0": "team-a",
		"team_access.0.team_id": 7, "team_access.0.role_id": 3,
	} {
		if got := d.Get(field); got != want {
			t.Errorf("%s: got %v, want %v", field, got, want)
		}
	}
	if d.Id() != "3:team-a" {
		t.Errorf("id: got %q", d.Id())
	}
}

func TestDataSourceKubernetesNamespaceAccess_Errors(t *testing.T) {
	old := NewMockServer(t)
	old.On("GET", "/system/version", RespondJSON(http.StatusOK, map[string]interface{}{
		"ServerVersion": "2.45.3", "ServerEdition": "BE",
	}))
	ds := dataSourceKubernetesNamespaceAccess()
	d := ds.TestResourceData()
	_ = d.Set("endpoint_id", 3)
	_ = d.Set("namespace_id", "team-a")
	if err := rcRead(ds, d, old.Client()); err == nil || !strings.Contains(err.Error(), "portainer_kubernetes_namespace_access requires") {
		t.Errorf("expected the requirement to be named, got %v", err)
	}

	unavailable := NewMockServer(t)
	unavailable.On("GET", "/endpoints/3/namespaces/team-a/access", RespondString(http.StatusServiceUnavailable, "", "not yet available"))
	if err := rcRead(ds, d, unavailable.Client()); err == nil || !strings.Contains(err.Error(), "503") {
		t.Errorf("expected the 503 to surface, got %v", err)
	}
}
