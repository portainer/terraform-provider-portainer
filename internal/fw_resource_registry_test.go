package internal

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// fakeRegistries is a stateful stand-in for Portainer's registry API: list,
// create, inspect, update, delete, the access of a registry on an environment
// and the configure call.
type fakeRegistries struct {
	mu         sync.Mutex
	nextID     int64
	registries map[int64]*portainerRegistry
	passwords  map[int64]string
}

func newFakeRegistries(m *MockServer) *fakeRegistries {
	f := &fakeRegistries{registries: map[int64]*portainerRegistry{}, passwords: map[int64]string{}}
	m.On("GET", "/registries", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		list := []*portainerRegistry{}
		for id := int64(1); id <= f.nextID; id++ {
			if reg, ok := f.registries[id]; ok {
				list = append(list, reg)
			}
		}
		RespondJSON(http.StatusOK, list)(w, r)
	})
	m.On("POST", "/registries", func(w http.ResponseWriter, r *http.Request) {
		var body registryPayload
		_ = json.NewDecoder(r.Body).Decode(&body)
		f.mu.Lock()
		defer f.mu.Unlock()
		f.nextID++
		reg := &portainerRegistry{
			ID: f.nextID, Name: body.Name, URL: body.URL, BaseURL: body.BaseURL, Authentication: body.Authentication, Username: body.Username,
			Quay: &registryOrganisation{}, Github: &registryOrganisation{}, Gitlab: &registryGitlab{}, Ecr: &registryEcr{},
		}
		if body.Type != nil {
			reg.Type = *body.Type
		}
		f.apply(reg, &body)
		if body.Gitlab != nil {
			reg.Gitlab = body.Gitlab
		}
		f.passwords[reg.ID] = body.Password
		f.registries[reg.ID] = reg
		RespondJSON(http.StatusOK, reg)(w, r)
	})
	for id := int64(1); id <= 10; id++ {
		id := id
		p := "/registries/" + strconv.FormatInt(id, 10)
		m.On("GET", p, func(w http.ResponseWriter, r *http.Request) {
			f.mu.Lock()
			defer f.mu.Unlock()
			reg, ok := f.registries[id]
			if !ok {
				RespondString(http.StatusNotFound, "application/json", `{"message":"not found"}`)(w, r)
				return
			}
			RespondJSON(http.StatusOK, reg)(w, r)
		})
		m.On("PUT", p, func(w http.ResponseWriter, r *http.Request) {
			var body registryPayload
			_ = json.NewDecoder(r.Body).Decode(&body)
			f.mu.Lock()
			defer f.mu.Unlock()
			reg, ok := f.registries[id]
			if !ok {
				RespondString(http.StatusNotFound, "application/json", `{"message":"not found"}`)(w, r)
				return
			}
			reg.Name, reg.URL, reg.BaseURL, reg.Authentication, reg.Username = body.Name, body.URL, body.BaseURL, body.Authentication, body.Username
			if body.Password != "" {
				f.passwords[id] = body.Password
			}
			f.apply(reg, &body)
			RespondJSON(http.StatusOK, reg)(w, r)
		})
		m.On("DELETE", p, func(w http.ResponseWriter, r *http.Request) {
			f.mu.Lock()
			defer f.mu.Unlock()
			if _, ok := f.registries[id]; !ok {
				RespondString(http.StatusNotFound, "application/json", `{"message":"not found"}`)(w, r)
				return
			}
			delete(f.registries, id)
			w.WriteHeader(http.StatusNoContent)
		})
		for eid := 1; eid <= 3; eid++ {
			eid := eid
			m.On("PUT", fmt.Sprintf("/endpoints/%d/registries/%d", eid, id), func(w http.ResponseWriter, r *http.Request) {
				var body struct {
					UserAccessPolicies map[string]accessPolicy `json:"userAccessPolicies"`
					TeamAccessPolicies map[string]accessPolicy `json:"teamAccessPolicies"`
					Namespaces         []string                `json:"namespaces"`
				}
				_ = json.NewDecoder(r.Body).Decode(&body)
				f.mu.Lock()
				defer f.mu.Unlock()
				reg, ok := f.registries[id]
				if !ok {
					RespondString(http.StatusNotFound, "application/json", `{"message":"not found"}`)(w, r)
					return
				}
				if reg.RegistryAccesses == nil {
					reg.RegistryAccesses = map[string]registryAccessPolicies{}
				}
				reg.RegistryAccesses[strconv.Itoa(eid)] = registryAccessPolicies{UserAccessPolicies: body.UserAccessPolicies, TeamAccessPolicies: body.TeamAccessPolicies, Namespaces: body.Namespaces}
				w.WriteHeader(http.StatusNoContent)
			})
		}
	}
	return f
}

func (f *fakeRegistries) apply(reg *portainerRegistry, body *registryPayload) {
	if body.Quay != nil {
		reg.Quay = body.Quay
	}
	if body.Github != nil {
		reg.Github = body.Github
	}
	if body.Ecr != nil {
		reg.Ecr = body.Ecr
	}
}

func (f *fakeRegistries) add(reg *portainerRegistry) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if reg.Quay == nil {
		reg.Quay, reg.Github, reg.Gitlab, reg.Ecr = &registryOrganisation{}, &registryOrganisation{}, &registryGitlab{}, &registryEcr{}
	}
	if reg.ID > f.nextID {
		f.nextID = reg.ID
	}
	f.registries[reg.ID] = reg
}

func registryConfig(m *MockServer, body string) string {
	return mockProviderConfig(m) + "resource \"portainer_registry\" \"r\" {\n" + body + "\n}\n"
}

// TestRegistryResource_Lifecycle: a username implies authentication, the
// password reaches Portainer but is never read back, an update sends the
// whole registry, import, and a type change replaces it.
func TestRegistryResource_Lifecycle(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	fake := newFakeRegistries(mock)
	base := `
  name     = "custom"
  url      = "registry.example.com"
  type     = 3
  username = "robot"
  password = "secret"
`
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{
				Config: registryConfig(mock, base),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("portainer_registry.r", "id", "1"),
					resource.TestCheckResourceAttr("portainer_registry.r", "authentication", "true"),
					resource.TestCheckNoResourceAttr("portainer_registry.r", "base_url"),
					func(*terraform.State) error {
						p := lastRequestBody(t, mock, "POST", "/registries")
						if p["name"] != "custom" || p["type"] != float64(3) || p["authentication"] != true || p["username"] != "robot" || p["password"] != "secret" {
							return fmt.Errorf("create payload: %v", p)
						}
						for _, k := range []string{"quay", "github", "gitlab", "ecr", "baseURL"} {
							if _, ok := p[k]; ok {
								return fmt.Errorf("a custom registry must not send %s: %v", k, p)
							}
						}
						return nil
					},
				),
			},
			{
				Config: registryConfig(mock, base+`  base_url = "https://registry.example.com"`),
				Check: func(*terraform.State) error {
					p := lastRequestBody(t, mock, "PUT", "/registries/1")
					if p["baseURL"] != "https://registry.example.com" || p["password"] != "secret" || p["authentication"] != true {
						return fmt.Errorf("update payload: %v", p)
					}
					if _, ok := p["type"]; ok {
						return fmt.Errorf("the update payload has no type: %v", p)
					}
					return nil
				},
			},
			{
				ResourceName:            "portainer_registry.r",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"password"},
			},
			{
				// Without a username nothing implies authentication any more.
				Config: registryConfig(mock, `
  name     = "custom"
  url      = "registry.example.com"
  type     = 3
`),
				Check: resource.TestCheckResourceAttr("portainer_registry.r", "authentication", "false"),
			},
			{
				PreConfig: func() { fake.mutate(1, func(r *portainerRegistry) { r.Name = "renamed" }) },
				Config: registryConfig(mock, `
  name     = "custom"
  url      = "registry.example.com"
  type     = 5
`),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction("portainer_registry.r", plancheck.ResourceActionDestroyBeforeCreate),
				}},
			},
		},
	})
}

func (f *fakeRegistries) mutate(id int64, fn func(*portainerRegistry)) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if reg, ok := f.registries[id]; ok {
		fn(reg)
	}
}

// TestRegistryResource_TypeSettings: each type sends its own settings on
// create and update; GitLab settings are only part of the create payload.
func TestRegistryResource_TypeSettings(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name, body, key, update string
		want                    string
	}{
		{"quay", `type = 1
  quay_use_organisation  = true
  quay_organisation_name = "acme"`, "quay", "quay", "map[OrganisationName:acme UseOrganisation:true]"},
		{"gitlab", `type = 4
  instance_url = "https://gitlab.example.com"`, "gitlab", "", "map[InstanceURL:https://gitlab.example.com]"},
		{"ecr", `type = 7
  aws_region = "eu-west-1"`, "ecr", "ecr", "map[Region:eu-west-1]"},
		{"github", `type = 8
  github_use_organisation  = true
  github_organisation_name = "acme"`, "github", "github", "map[OrganisationName:acme UseOrganisation:true]"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mock := NewMockServer(t)
			newFakeRegistries(mock)
			cfg := func(name string) string {
				return registryConfig(mock, fmt.Sprintf("  name = %q\n  url = \"r.example.com\"\n  %s", name, tc.body))
			}
			resource.UnitTest(t, resource.TestCase{
				ProtoV5ProviderFactories: testProtoV5Factories,
				Steps: []resource.TestStep{
					{
						Config: cfg(tc.name),
						Check: func(*terraform.State) error {
							if p := lastRequestBody(t, mock, "POST", "/registries"); fmt.Sprint(p[tc.key]) != tc.want {
								return fmt.Errorf("create %s: %v", tc.key, p)
							}
							return nil
						},
					},
					{
						Config: cfg(tc.name + "-renamed"),
						Check: func(*terraform.State) error {
							p := lastRequestBody(t, mock, "PUT", "/registries/1")
							if tc.update == "" {
								if _, ok := p[tc.key]; ok {
									return fmt.Errorf("the update payload has no %s settings: %v", tc.key, p)
								}
								return nil
							}
							if fmt.Sprint(p[tc.update]) != tc.want {
								return fmt.Errorf("update %s: %v", tc.update, p)
							}
							return nil
						},
					},
				},
			})
		})
	}
}

// TestRegistryResource_ExistingName: a registry of the same name is adopted
// and updated instead of created.
func TestRegistryResource_ExistingName(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	fake := newFakeRegistries(mock)
	fake.add(&portainerRegistry{ID: 4, Name: "hub", Type: 6, URL: "docker.io"})
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{{
			Config: registryConfig(mock, `
  name = "hub"
  url  = "docker.io"
  type = 6
`),
			Check: resource.ComposeTestCheckFunc(
				resource.TestCheckResourceAttr("portainer_registry.r", "id", "4"),
				func(*terraform.State) error {
					if mock.FindRequest("POST", "/registries") != nil {
						return fmt.Errorf("an existing registry must not be created again")
					}
					if mock.FindRequest("PUT", "/registries/4") == nil {
						return fmt.Errorf("expected the existing registry to be updated")
					}
					return nil
				},
			),
		}},
	})
}

// TestRegistryResource_WriteOnlyPassword: password_wo is sent on create and
// when its version changes, and never stored.
func TestRegistryResource_WriteOnlyPassword(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	fake := newFakeRegistries(mock)
	cfg := func(pw string, version int) string {
		return registryConfig(mock, fmt.Sprintf(`
  name                = "custom"
  url                 = "registry.example.com"
  type                = 3
  username            = "robot"
  password_wo         = %q
  password_wo_version = %d
`, pw, version))
	}
	password := func() string {
		fake.mu.Lock()
		defer fake.mu.Unlock()
		return fake.passwords[1]
	}
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{
				Config: cfg("first", 1),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckNoResourceAttr("portainer_registry.r", "password_wo"),
					resource.TestCheckNoResourceAttr("portainer_registry.r", "password"),
					func(*terraform.State) error {
						if got := password(); got != "first" {
							return fmt.Errorf("expected the write-only password on create, got %q", got)
						}
						return nil
					},
				),
			},
			{Config: cfg("second", 1), PlanOnly: true},
			{
				Config: cfg("second", 2),
				Check: func(*terraform.State) error {
					if got := password(); got != "second" {
						return fmt.Errorf("expected the new password after the version change, got %q", got)
					}
					return nil
				},
			},
		},
	})
}

func TestRegistryResource_Errors(t *testing.T) {
	t.Parallel()
	t.Run("create", func(t *testing.T) {
		mock := NewMockServer(t)
		mock.On("POST", "/registries", RespondString(http.StatusConflict, "application/json", `{"message":"exists"}`))
		newFakeRegistries(mock)
		resource.UnitTest(t, resource.TestCase{
			ProtoV5ProviderFactories: testProtoV5Factories,
			Steps: []resource.TestStep{{
				Config:      registryConfig(mock, "name = \"x\"\n url = \"y\"\n type = 3"),
				ExpectError: regexp.MustCompile(`failed to create registry`),
			}},
		})
	})
	t.Run("validation", func(t *testing.T) {
		mock := NewMockServer(t)
		resource.UnitTest(t, resource.TestCase{
			ProtoV5ProviderFactories: testProtoV5Factories,
			Steps: []resource.TestStep{
				{Config: registryConfig(mock, "name = \"\"\n url = \"y\"\n type = 3"), ExpectError: regexp.MustCompile(`at least 1`)},
				{Config: registryConfig(mock, "name = \"x\"\n url = \"y\"\n type = 9"), ExpectError: regexp.MustCompile(`between 1 and 8`)},
				{Config: registryConfig(mock, "name = \"x\"\n url = \"y\"\n type = 3\n password = \"a\"\n password_wo = \"b\"\n password_wo_version = 1"), ExpectError: regexp.MustCompile(`cannot be specified`)},
			},
		})
	})
	t.Run("gone", func(t *testing.T) {
		mock := NewMockServer(t)
		fake := newFakeRegistries(mock)
		resource.UnitTest(t, resource.TestCase{
			ProtoV5ProviderFactories: testProtoV5Factories,
			Steps: []resource.TestStep{
				{Config: registryConfig(mock, "name = \"x\"\n url = \"y\"\n type = 3")},
				{
					PreConfig:          func() { fake.mu.Lock(); delete(fake.registries, 1); fake.mu.Unlock() },
					Config:             registryConfig(mock, "name = \"x\"\n url = \"y\"\n type = 3"),
					PlanOnly:           true,
					ExpectNonEmptyPlan: true,
				},
			},
		})
	})
}

func TestRegistryResource_UpgradeFromV300(t *testing.T) {
	t.Parallel()
	for name, body := range map[string]string{
		"minimal":     "name = \"r\"\n url = \"registry.example.com\"\n type = 3",
		"credentials": "name = \"r\"\n url = \"registry.example.com\"\n type = 3\n username = \"robot\"\n password = \"secret\"\n base_url = \"https://registry.example.com\"",
		"no-auth":     "name = \"r\"\n url = \"registry.example.com\"\n type = 6\n authentication = false",
		"quay":        "name = \"r\"\n url = \"quay.io\"\n type = 1\n quay_use_organisation = true\n quay_organisation_name = \"acme\"",
		"gitlab":      "name = \"r\"\n url = \"registry.gitlab.com\"\n type = 4\n instance_url = \"https://gitlab.com\"",
		"ecr":         "name = \"r\"\n url = \"1.dkr.ecr.eu-west-1.amazonaws.com\"\n type = 7\n aws_region = \"eu-west-1\"",
		"github":      "name = \"r\"\n url = \"ghcr.io\"\n type = 8\n github_use_organisation = true\n github_organisation_name = \"acme\"",
	} {
		t.Run(name, func(t *testing.T) {
			mock := NewMockServer(t)
			newFakeRegistries(mock)
			config := registryConfig(mock, body)
			resource.Test(t, resource.TestCase{
				IsUnitTest: true,
				Steps: []resource.TestStep{
					{ExternalProviders: previousRelease, Config: config},
					{
						ProtoV5ProviderFactories: testProtoV5Factories,
						Config:                   config,
						ConfigPlanChecks:         resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}},
					},
				},
			})
		})
	}
}

// TestAccRegistryResource runs against a real Portainer: a registry needs
// nothing but a URL.
func TestAccRegistryResource(t *testing.T) {
	config := fmt.Sprintf(`
resource "portainer_registry" "r" {
  name = %q
  url  = "registry.example.com"
  type = 3
}

data "portainer_registry" "r" {
  name       = portainer_registry.r.name
  depends_on = [portainer_registry.r]
}
`, acctest.RandomWithPrefix("acc-registry"))
	resource.Test(t, resource.TestCase{
		PreCheck: func() { accPreCheck(t) },
		Steps: []resource.TestStep{
			{ExternalProviders: previousRelease, Config: accProvider() + config},
			{
				ProtoV5ProviderFactories: testProtoV5Factories,
				Config:                   accProvider() + config,
				ConfigPlanChecks:         resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}},
				Check:                    resource.TestCheckResourceAttrPair("data.portainer_registry.r", "id", "portainer_registry.r", "id"),
			},
			{
				ProtoV5ProviderFactories: testProtoV5Factories,
				Config:                   accProvider() + config,
				ResourceName:             "portainer_registry.r",
				ImportState:              true,
				ImportStateVerify:        true,
				// v3.0.0 stored "" for these when unset, an import leaves
				// them null; both plan no change.
				ImportStateVerifyIgnore: []string{"aws_region", "base_url", "github_organisation_name", "instance_url", "quay_organisation_name", "username"},
			},
		},
	})
}

// --- portainer_registry_access ---

func registryAccessConfig(m *MockServer, principal string, role int) string {
	return mockProviderConfig(m) + fmt.Sprintf(`
resource "portainer_registry_access" "a" {
  registry_id = 1
  endpoint_id = 2
  %s
  role_id     = %d
}
`, principal, role)
}

// TestRegistryAccessResource_Team: the policy joins the ones already on the
// environment, a role change rewrites it, import accepts the current and the
// legacy ID, and destroy removes only this principal.
func TestRegistryAccessResource_Team(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	fake := newFakeRegistries(mock)
	fake.add(&portainerRegistry{ID: 1, Name: "hub", Type: 6, RegistryAccesses: map[string]registryAccessPolicies{
		"2": {UserAccessPolicies: map[string]accessPolicy{"9": {RoleID: 1}}, TeamAccessPolicies: map[string]accessPolicy{}, Namespaces: []string{"ns"}},
	}})
	policies := func() registryAccessPolicies {
		fake.mu.Lock()
		defer fake.mu.Unlock()
		return fake.registries[1].RegistryAccesses["2"]
	}
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		CheckDestroy: func(*terraform.State) error {
			p := policies()
			if _, ok := p.TeamAccessPolicies["7"]; ok {
				return fmt.Errorf("the team policy must be removed: %v", p)
			}
			if _, ok := p.UserAccessPolicies["9"]; !ok {
				return fmt.Errorf("other policies must stay: %v", p)
			}
			return nil
		},
		Steps: []resource.TestStep{
			{
				Config: registryAccessConfig(mock, "team_id = 7", 2),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("portainer_registry_access.a", "id", "1:2:team:7"),
					resource.TestCheckNoResourceAttr("portainer_registry_access.a", "user_id"),
					func(*terraform.State) error {
						p := policies()
						if p.TeamAccessPolicies["7"].RoleID != 2 || p.UserAccessPolicies["9"].RoleID != 1 || fmt.Sprint(p.Namespaces) != "[ns]" {
							return fmt.Errorf("policies: %+v", p)
						}
						return nil
					},
				),
			},
			{
				Config: registryAccessConfig(mock, "team_id = 7", 3),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction("portainer_registry_access.a", plancheck.ResourceActionUpdate),
				}},
				Check: func(*terraform.State) error {
					if p := policies(); p.TeamAccessPolicies["7"].RoleID != 3 {
						return fmt.Errorf("role not updated: %+v", p)
					}
					return nil
				},
			},
			{ResourceName: "portainer_registry_access.a", ImportState: true, ImportStateVerify: true},
			{ResourceName: "portainer_registry_access.a", ImportState: true, ImportStateId: "1/2/team/7", ImportStateVerify: true},
			{ResourceName: "portainer_registry_access.a", ImportState: true, ImportStateId: "1/2/group/7", ExpectError: regexp.MustCompile(`invalid import ID`)},
			{ResourceName: "portainer_registry_access.a", ImportState: true, ImportStateId: "1:2", ExpectError: regexp.MustCompile(`(?i)import ID`)},
			{ResourceName: "portainer_registry_access.a", ImportState: true, ImportStateId: "x/2/team/7", ExpectError: regexp.MustCompile(`invalid import ID`)},
			{
				// Removed in Portainer: planned again.
				PreConfig: func() {
					fake.mu.Lock()
					delete(fake.registries[1].RegistryAccesses["2"].TeamAccessPolicies, "7")
					fake.mu.Unlock()
				},
				Config:             registryAccessConfig(mock, "team_id = 7", 3),
				PlanOnly:           true,
				ExpectNonEmptyPlan: true,
			},
		},
	})
}

func TestRegistryAccessResource_User(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	fake := newFakeRegistries(mock)
	fake.add(&portainerRegistry{ID: 1, Name: "hub", Type: 6})
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{
				Config: registryAccessConfig(mock, "user_id = 9", 1),
				Check:  resource.TestCheckResourceAttr("portainer_registry_access.a", "id", "1:2:user:9"),
			},
			{
				// The registry is gone: the access goes with it.
				PreConfig:          func() { fake.mu.Lock(); delete(fake.registries, 1); fake.mu.Unlock() },
				Config:             registryAccessConfig(mock, "user_id = 9", 1),
				PlanOnly:           true,
				ExpectNonEmptyPlan: true,
			},
		},
	})
}

func TestRegistryAccessResource_Errors(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	fake := newFakeRegistries(mock)
	fake.add(&portainerRegistry{ID: 1, Name: "hub", Type: 6})
	mock.mu.Lock()
	mock.routes = append([]route{{method: "PUT", path: "/endpoints/3/registries/1", handler: RespondString(http.StatusInternalServerError, "application/json", `{"message":"boom"}`)}}, mock.routes...)
	mock.mu.Unlock()
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{Config: registryAccessConfig(mock, "", 1), ExpectError: regexp.MustCompile(`either team_id or user_id must be provided`)},
			{
				Config:      strings.Replace(registryAccessConfig(mock, "team_id = 7", 1), "endpoint_id = 2", "endpoint_id = 3", 1),
				ExpectError: regexp.MustCompile(`failed to update registry access`),
			},
		},
	})
}

func TestRegistryAccessResource_UpgradeFromV300(t *testing.T) {
	t.Parallel()
	for name, principal := range map[string]string{"team": "team_id = 7", "user": "user_id = 9"} {
		t.Run(name, func(t *testing.T) {
			mock := NewMockServer(t)
			fake := newFakeRegistries(mock)
			fake.add(&portainerRegistry{ID: 1, Name: "hub", Type: 6})
			for _, config := range []string{registryAccessConfig(mock, principal, 0)} {
				resource.Test(t, resource.TestCase{
					IsUnitTest: true,
					Steps: []resource.TestStep{
						{ExternalProviders: previousRelease, Config: config},
						{
							ProtoV5ProviderFactories: testProtoV5Factories,
							Config:                   config,
							ConfigPlanChecks:         resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}},
						},
					},
				})
			}
		})
	}
}

// --- portainer_registry_configure ---

// TestRegistryConfigureResource: the payload uses PascalCase keys and base64
// TLS material, unset fields are left out, the write-only twins are sent, and
// neither refresh nor destroy calls the API.
func TestRegistryConfigureResource(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	var mu sync.Mutex
	var bodies []map[string]interface{}
	mock.On("POST", "/registries/4/configure", func(w http.ResponseWriter, r *http.Request) {
		var body map[string]interface{}
		_ = json.NewDecoder(r.Body).Decode(&body)
		mu.Lock()
		bodies = append(bodies, body)
		mu.Unlock()
		w.WriteHeader(http.StatusNoContent)
	})
	last := func() map[string]interface{} {
		mu.Lock()
		defer mu.Unlock()
		return bodies[len(bodies)-1]
	}
	b64 := func(s string) string { return base64.StdEncoding.EncodeToString([]byte(s)) }
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		CheckDestroy: func(*terraform.State) error {
			for _, r := range mock.Requests() {
				if r.Method != http.MethodPost {
					return fmt.Errorf("unexpected %s %s", r.Method, r.Path)
				}
			}
			return nil
		},
		Steps: []resource.TestStep{
			{
				Config: mockProviderConfig(mock) + `
resource "portainer_registry_configure" "c" {
  registry_id     = 4
  authentication  = true
  username        = "robot"
  password        = "secret"
  tls             = true
  tls_ca_cert     = "-----BEGIN CERTIFICATE-----\nca\n-----END CERTIFICATE-----"
}
`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("portainer_registry_configure.c", "id", "4/configure"),
					resource.TestCheckResourceAttr("portainer_registry_configure.c", "tls_skip_verify", "false"),
					func(*terraform.State) error {
						p := last()
						if p["Authentication"] != true || p["Username"] != "robot" || p["Password"] != "secret" || p["TLS"] != true || p["TLSSkipVerify"] != false {
							return fmt.Errorf("payload: %v", p)
						}
						if p["TLSCACertFile"] != b64("-----BEGIN CERTIFICATE-----\nca\n-----END CERTIFICATE-----") {
							return fmt.Errorf("TLSCACertFile must be the base64 PEM, got %v", p["TLSCACertFile"])
						}
						for _, k := range []string{"Region", "TLSCertFile", "TLSKeyFile"} {
							if _, ok := p[k]; ok {
								return fmt.Errorf("%s must be omitted when it is not configured: %v", k, p)
							}
						}
						return nil
					},
				),
			},
			{
				Config: mockProviderConfig(mock) + `
resource "portainer_registry_configure" "c" {
  registry_id         = 4
  authentication      = true
  username            = "robot"
  tls                 = true
  password_wo         = "registry-pass"
  password_wo_version = 1
  tls_key_wo          = "PEM KEY"
  tls_key_wo_version  = 1
  region              = "eu-west-1"
}
`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckNoResourceAttr("portainer_registry_configure.c", "password"),
					resource.TestCheckNoResourceAttr("portainer_registry_configure.c", "password_wo"),
					func(*terraform.State) error {
						p := last()
						if p["Password"] != "registry-pass" || p["TLSKeyFile"] != b64("PEM KEY") || p["Region"] != "eu-west-1" {
							return fmt.Errorf("expected the write-only values: %v", p)
						}
						return nil
					},
				),
			},
		},
	})
}

func TestRegistryConfigureResource_UpgradeFromV300(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("POST", "/registries/4/configure", RespondString(http.StatusNoContent, "", ""))
	for _, body := range []string{
		"registry_id = 4\n authentication = false",
		"registry_id = 4\n authentication = true\n username = \"robot\"\n password = \"secret\"\n region = \"eu-west-1\"\n tls = true\n tls_skip_verify = true\n tls_ca_cert = \"CA\"\n tls_cert = \"CERT\"\n tls_key = \"KEY\"",
	} {
		config := mockProviderConfig(mock) + "resource \"portainer_registry_configure\" \"c\" {\n" + body + "\n}\n"
		resource.Test(t, resource.TestCase{
			IsUnitTest: true,
			Steps: []resource.TestStep{
				{ExternalProviders: previousRelease, Config: config},
				{
					ProtoV5ProviderFactories: testProtoV5Factories,
					Config:                   config,
					ConfigPlanChecks:         resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}},
				},
			},
		})
	}
}

// --- data sources ---

func TestRegistryDataSources(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	fake := newFakeRegistries(mock)
	fake.add(&portainerRegistry{ID: 1, Name: "hub", Type: 6, URL: "docker.io", RegistryAccesses: map[string]registryAccessPolicies{
		"2": {UserAccessPolicies: map[string]accessPolicy{"9": {RoleID: 4}}, TeamAccessPolicies: map[string]accessPolicy{"7": {RoleID: 3}}},
	}})
	mock.On("GET", "/tags", RespondJSON(http.StatusOK, []map[string]interface{}{{"id": 5, "Name": "prod"}}))
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{
				Config: mockProviderConfig(mock) + `
data "portainer_registry" "r" { name = "hub" }
data "portainer_registry_access" "team" {
  registry_id = 1
  endpoint_id = 2
  team_id     = 7
}
data "portainer_registry_access" "user" {
  registry_id = 1
  endpoint_id = 2
  user_id     = 9
}
data "portainer_tag" "t" { name = "prod" }
`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("data.portainer_registry.r", "id", "1"),
					resource.TestCheckResourceAttr("data.portainer_registry.r", "url", "docker.io"),
					resource.TestCheckResourceAttr("data.portainer_registry.r", "type", "6"),
					resource.TestCheckResourceAttr("data.portainer_registry_access.team", "id", "1/2/team/7"),
					resource.TestCheckResourceAttr("data.portainer_registry_access.team", "role_id", "3"),
					resource.TestCheckResourceAttr("data.portainer_registry_access.user", "id", "1/2/user/9"),
					resource.TestCheckResourceAttr("data.portainer_registry_access.user", "role_id", "4"),
					resource.TestCheckResourceAttr("data.portainer_tag.t", "id", "5"),
				),
			},
			{Config: mockProviderConfig(mock) + `data "portainer_registry" "r" { name = "missing" }`, ExpectError: regexp.MustCompile(`registry missing not found`)},
			{Config: mockProviderConfig(mock) + `data "portainer_tag" "t" { name = "missing" }`, ExpectError: regexp.MustCompile(`tag missing not found`)},
			{
				Config:      mockProviderConfig(mock) + "data \"portainer_registry_access\" \"a\" {\n registry_id = 1\n endpoint_id = 2\n}",
				ExpectError: regexp.MustCompile(`either team_id or user_id must be provided`),
			},
			{
				Config:      mockProviderConfig(mock) + "data \"portainer_registry_access\" \"a\" {\n registry_id = 1\n endpoint_id = 2\n team_id = 8\n}",
				ExpectError: regexp.MustCompile(`access policy not found`),
			},
			{
				Config:      mockProviderConfig(mock) + "data \"portainer_registry_access\" \"a\" {\n registry_id = 5\n endpoint_id = 2\n team_id = 7\n}",
				ExpectError: regexp.MustCompile(`registry 5 not found`),
			},
		},
	})
}

func TestRegistryConnectionDataSource(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	var mu sync.Mutex
	result := map[string]interface{}{"success": true, "message": "registry is reachable"}
	mock.On("POST", "/registries/ping", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		RespondJSON(http.StatusOK, result)(w, r)
	})
	cfg := func(extra string) string {
		return mockProviderConfig(mock) + fmt.Sprintf(`
data "portainer_registry_connection" "c" {
  url      = "registry.example.com"
  type     = 8
  username = "robot"
  password = "secret"
  %s
}
`, extra)
	}
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			// Portainer rejects a ping without credentials.
			{Config: strings.Replace(cfg(""), `password = "secret"`, "", 1), ExpectError: regexp.MustCompile(`(?i)password.*required|required.*password`)},
			{
				Config: cfg("tls = true"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("data.portainer_registry_connection.c", "id", "registry-ping-registry.example.com"),
					resource.TestCheckResourceAttr("data.portainer_registry_connection.c", "success", "true"),
					resource.TestCheckResourceAttr("data.portainer_registry_connection.c", "fail_on_error", "false"),
					func(*terraform.State) error {
						p := lastRequestBody(t, mock, "POST", "/registries/ping")
						for k, want := range map[string]interface{}{"URL": "registry.example.com", "Type": float64(8), "Username": "robot", "Password": "secret", "TLS": true} {
							if p[k] != want {
								return fmt.Errorf("payload.%s: expected %v, got %v", k, want, p[k])
							}
						}
						return nil
					},
				),
			},
			{
				// An unreachable registry is reported, not raised...
				PreConfig: func() {
					mu.Lock()
					result = map[string]interface{}{"success": false, "message": "unauthorized"}
					mu.Unlock()
				},
				Config: cfg(""),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("data.portainer_registry_connection.c", "success", "false"),
					resource.TestCheckResourceAttr("data.portainer_registry_connection.c", "message", "unauthorized"),
					resource.TestCheckResourceAttr("data.portainer_registry_connection.c", "tls", "false"),
				),
			},
			// ...unless asked for.
			{Config: cfg("fail_on_error = true"), ExpectError: regexp.MustCompile(`unauthorized`)},
			{Config: strings.Replace(cfg(""), "type     = 8", "type     = 9", 1), ExpectError: regexp.MustCompile(`between 1 and 8`)},
			{Config: cfg("")},
		},
	})
}

// TestAccRegistryAccessResource needs an environment to grant the access on
// (PORTAINER_ACC_SWARM_ENV_ID) and Business Edition, whose registry access
// policies are per team (PORTAINER_ACC_BE=1).
func TestAccRegistryAccessResource(t *testing.T) {
	envID := os.Getenv("PORTAINER_ACC_SWARM_ENV_ID")
	if envID == "" || os.Getenv("PORTAINER_ACC_BE") != "1" {
		t.Skip("PORTAINER_ACC_SWARM_ENV_ID and PORTAINER_ACC_BE=1 are needed for registry access policies")
	}
	config := fmt.Sprintf(`
resource "portainer_team" "t" { name = %[1]q }

resource "portainer_registry" "r" {
  name = %[1]q
  url  = "registry.example.com"
  type = 3
}

resource "portainer_registry_access" "a" {
  registry_id = portainer_registry.r.id
  endpoint_id = %[2]s
  team_id     = portainer_team.t.id
}
`, acctest.RandomWithPrefix("acc-regaccess"), envID)
	resource.Test(t, resource.TestCase{
		PreCheck: func() { accPreCheck(t) },
		Steps: []resource.TestStep{
			{ExternalProviders: previousRelease, Config: accProvider() + config},
			{
				ProtoV5ProviderFactories: testProtoV5Factories,
				Config:                   accProvider() + config,
				ConfigPlanChecks:         resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}},
			},
		},
	})
}

// TestAccRegistryConfigureResource needs Business Edition, whose registry
// management the configure call belongs to (PORTAINER_ACC_BE=1).
func TestAccRegistryConfigureResource(t *testing.T) {
	if os.Getenv("PORTAINER_ACC_BE") != "1" {
		t.Skip("PORTAINER_ACC_BE=1 is needed for registry management")
	}
	config := fmt.Sprintf(`
resource "portainer_registry" "r" {
  name = %q
  url  = "registry.example.com"
  type = 3
}

resource "portainer_registry_configure" "c" {
  registry_id    = portainer_registry.r.id
  authentication = false
}
`, acctest.RandomWithPrefix("acc-regconf"))
	resource.Test(t, resource.TestCase{
		PreCheck: func() { accPreCheck(t) },
		Steps: []resource.TestStep{
			{ExternalProviders: previousRelease, Config: accProvider() + config},
			{
				ProtoV5ProviderFactories: testProtoV5Factories,
				Config:                   accProvider() + config,
				ConfigPlanChecks:         resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}},
			},
		},
	})
}
