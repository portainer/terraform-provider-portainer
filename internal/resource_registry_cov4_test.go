package internal

import (
	"net/http"
	"testing"
)

// TestRegistryCov4_Create_ExistingNameUpdates covers the branch of Create where
// findRegistryByName finds an existing registry with the same name: instead of
// POSTing a new one, the resource adopts the existing ID and chains into Update
// (PUT) followed by Read.
func TestRegistryCov4_Create_ExistingNameUpdates(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/registries", RespondJSON(http.StatusOK, []map[string]interface{}{
		{"Id": 9, "Name": "exists"},
	}))
	mock.On("PUT", "/registries/9", RespondJSON(http.StatusOK, map[string]interface{}{"Id": 9}))
	mock.On("GET", "/registries/9", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id":             9,
		"Name":           "exists",
		"URL":            "https://example.com",
		"Type":           3,
		"Authentication": false,
	}))

	r := resourceRegistry()
	d := r.TestResourceData()
	_ = d.Set("name", "exists")
	_ = d.Set("url", "https://example.com")
	_ = d.Set("type", 3)

	if err := rcCreate(r, d, mock.Client()); err != nil {
		t.Fatalf("Create (existing name) failed: %v", err)
	}
	if d.Id() != "9" {
		t.Errorf("expected adopted ID %q, got %q", "9", d.Id())
	}
	if mock.FindRequest("PUT", "/registries/9") == nil {
		t.Error("expected PUT /registries/9 (update of adopted registry)")
	}
	// No POST should have been made for a create.
	if mock.FindRequest("POST", "/registries") != nil {
		t.Error("did not expect a create POST when name already exists")
	}
}

// TestRegistryCov4_Create_TypeSwitches drives Create for each registry type
// whose switch adds a nested payload struct (Quay=1, GitLab=4, ECR=7,
// GitHub=8), with authentication enabled to also cover the username/password
// branch.
func TestRegistryCov4_Create_TypeSwitches(t *testing.T) {
	cases := []struct {
		name         string
		registryType int
	}{
		{"quay", 1},
		{"gitlab", 4},
		{"ecr", 7},
		{"github", 8},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mock := NewMockServer(t)

			mock.On("GET", "/registries", RespondJSON(http.StatusOK, []map[string]interface{}{}))
			mock.On("POST", "/registries", RespondJSON(http.StatusOK, map[string]interface{}{
				"Id":   20,
				"Name": tc.name,
				"Type": tc.registryType,
			}))
			mock.On("GET", "/registries/20", RespondJSON(http.StatusOK, map[string]interface{}{
				"Id":             20,
				"Name":           tc.name,
				"URL":            "registry.example.com",
				"Type":           tc.registryType,
				"Authentication": true,
				"Username":       "svc",
			}))

			r := resourceRegistry()
			d := r.TestResourceData()
			_ = d.Set("name", tc.name)
			_ = d.Set("url", "registry.example.com")
			_ = d.Set("type", tc.registryType)
			_ = d.Set("authentication", true)
			_ = d.Set("username", "svc")
			_ = d.Set("password", "secret")
			_ = d.Set("quay_use_organisation", true)
			_ = d.Set("quay_organisation_name", "quayorg")
			_ = d.Set("instance_url", "https://gitlab.example.com")
			_ = d.Set("aws_region", "eu-central-1")
			_ = d.Set("github_use_organisation", true)
			_ = d.Set("github_organisation_name", "ghorg")

			if err := rcCreate(r, d, mock.Client()); err != nil {
				t.Fatalf("Create type %d failed: %v", tc.registryType, err)
			}
			if d.Id() != "20" {
				t.Errorf("expected ID %q, got %q", "20", d.Id())
			}
			if mock.FindRequest("POST", "/registries") == nil {
				t.Error("expected POST /registries")
			}
		})
	}
}

// TestRegistryCov4_Update_TypeSwitches drives Update for the types whose update
// switch adds a nested payload struct (Quay=1, ECR=7, GitHub=8).
func TestRegistryCov4_Update_TypeSwitches(t *testing.T) {
	cases := []struct {
		name         string
		registryType int
	}{
		{"quay", 1},
		{"ecr", 7},
		{"github", 8},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mock := NewMockServer(t)

			mock.On("PUT", "/registries/30", RespondJSON(http.StatusOK, map[string]interface{}{"Id": 30}))
			mock.On("GET", "/registries/30", RespondJSON(http.StatusOK, map[string]interface{}{
				"Id":             30,
				"Name":           tc.name,
				"URL":            "registry.example.com",
				"Type":           tc.registryType,
				"Authentication": true,
				"Username":       "svc",
			}))

			r := resourceRegistry()
			d := r.TestResourceData()
			d.SetId("30")
			_ = d.Set("name", tc.name)
			_ = d.Set("url", "registry.example.com")
			_ = d.Set("type", tc.registryType)
			_ = d.Set("authentication", true)
			_ = d.Set("username", "svc")
			_ = d.Set("password", "secret")
			_ = d.Set("quay_use_organisation", true)
			_ = d.Set("quay_organisation_name", "quayorg")
			_ = d.Set("aws_region", "eu-central-1")
			_ = d.Set("github_use_organisation", true)
			_ = d.Set("github_organisation_name", "ghorg")

			if err := rcUpdate(r, d, mock.Client()); err != nil {
				t.Fatalf("Update type %d failed: %v", tc.registryType, err)
			}
			if mock.FindRequest("PUT", "/registries/30") == nil {
				t.Error("expected PUT /registries/30")
			}
		})
	}
}

// TestRegistryCov4_Delete_404IsSuccess covers the RegistryDeleteNotFound branch:
// a 404 on delete is treated as success (the registry is already gone).
func TestRegistryCov4_Delete_404IsSuccess(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("DELETE", "/registries/40", RespondString(
		http.StatusNotFound, "application/json", `{"message":"not found"}`,
	))

	r := resourceRegistry()
	d := r.TestResourceData()
	d.SetId("40")

	if err := rcDelete(r, d, mock.Client()); err != nil {
		t.Fatalf("Delete should swallow 404, got error: %v", err)
	}
}

// TestRegistryCov4_Delete_Status200IsSuccess covers the branch where Portainer
// returns HTTP 200 (rather than the SDK-expected 204): the resource inspects
// the error string for "status 200" and treats it as success.
func TestRegistryCov4_Delete_Status200IsSuccess(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("DELETE", "/registries/41", RespondString(
		http.StatusOK, "application/json", `{}`,
	))

	r := resourceRegistry()
	d := r.TestResourceData()
	d.SetId("41")

	if err := rcDelete(r, d, mock.Client()); err != nil {
		t.Fatalf("Delete should treat status 200 as success, got error: %v", err)
	}
}
