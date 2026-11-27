package internal

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"sync"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// TestTeamResource_Lifecycle: create, rename in place, import, destroy.
func TestTeamResource_Lifecycle(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	fake := newFakeUsersAPI(mock)
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{
				Config: mockProviderConfig(mock) + `resource "portainer_team" "t" { name = "devops" }`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("portainer_team.t", "id", "1"),
					resource.TestCheckResourceAttr("portainer_team.t", "name", "devops"),
					func(*terraform.State) error {
						var body map[string]interface{}
						if err := mock.FindRequest("POST", "/teams").DecodeJSON(&body); err != nil || body["name"] != "devops" {
							return fmt.Errorf("unexpected create payload %v (%w)", body, err)
						}
						return nil
					},
				),
			},
			{
				Config: mockProviderConfig(mock) + `resource "portainer_team" "t" { name = "platform" }`,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction("portainer_team.t", plancheck.ResourceActionUpdate)},
				},
				Check: func(*terraform.State) error {
					var body map[string]interface{}
					if err := mock.FindRequest("PUT", "/teams/1").DecodeJSON(&body); err != nil || body["name"] != "platform" {
						return fmt.Errorf("unexpected update payload %v (%w)", body, err)
					}
					return nil
				},
			},
			{ResourceName: "portainer_team.t", ImportState: true, ImportStateVerify: true},
		},
	})
	if len(fake.teams) != 0 {
		t.Error("expected the team deleted on destroy")
	}
}

// TestTeamResource_AdoptsExistingTeam: a team with the same name is updated
// instead of created.
func TestTeamResource_AdoptsExistingTeam(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	fake := newFakeUsersAPI(mock)
	fake.addTeam("existing")
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{{
			Config: mockProviderConfig(mock) + `resource "portainer_team" "t" { name = "existing" }`,
			Check: resource.ComposeTestCheckFunc(
				resource.TestCheckResourceAttr("portainer_team.t", "id", "1"),
				func(*terraform.State) error {
					if mock.FindRequest("POST", "/teams") != nil {
						return fmt.Errorf("an existing team must not be created again")
					}
					if mock.FindRequest("PUT", "/teams/1") == nil {
						return fmt.Errorf("expected the existing team updated")
					}
					return nil
				},
			),
		}},
	})
}

func TestTeamResource_GoneOutsideTerraform(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	fake := newFakeUsersAPI(mock)
	config := mockProviderConfig(mock) + `resource "portainer_team" "t" { name = "devops" }`
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{Config: config},
			{
				PreConfig: func() {
					fake.mu.Lock()
					delete(fake.teams, 1)
					fake.mu.Unlock()
				},
				Config:             config,
				PlanOnly:           true,
				ExpectNonEmptyPlan: true,
			},
		},
	})
}

func TestTeamResource_EmptyNameRejected(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{{
			Config:      mockProviderConfig(mock) + `resource "portainer_team" "t" { name = "" }`,
			ExpectError: regexp.MustCompile(`at least 1`),
		}},
	})
}

func TestTeamResource_UpgradeFromV300(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	newFakeUsersAPI(mock)
	config := mockProviderConfig(mock) + `resource "portainer_team" "t" { name = "devops" }`
	resource.Test(t, resource.TestCase{
		IsUnitTest: true,
		Steps: []resource.TestStep{
			{ExternalProviders: previousRelease, Config: config},
			{
				ProtoV5ProviderFactories: testProtoV5Factories,
				Config:                   config,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
		},
	})
}

// TestAccTeamResource_UpgradeFromSDKv2 runs against a real Portainer: a team
// and a membership of a new user, created by v3.0.0, plan no change with this
// build and import cleanly.
func TestAccTeamResource_UpgradeFromSDKv2(t *testing.T) {
	name := acctest.RandomWithPrefix("acc-team")
	config := fmt.Sprintf(`
resource "portainer_team" "t" { name = %[1]q }

resource "portainer_user" "u" {
  username = %[1]q
  password = "Acc-Test-Password-123"
}

resource "portainer_team_membership" "m" {
  team_id = portainer_team.t.id
  user_id = portainer_user.u.id
  role    = 2
}`, name)
	resource.Test(t, resource.TestCase{
		PreCheck: func() { accPreCheck(t) },
		Steps: []resource.TestStep{
			{ExternalProviders: previousRelease, Config: accProvider() + config},
			{
				ProtoV5ProviderFactories: testProtoV5Factories,
				Config:                   accProvider() + config,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
			{
				ProtoV5ProviderFactories: testProtoV5Factories,
				Config:                   accProvider() + config,
				ResourceName:             "portainer_team.t",
				ImportState:              true,
				ImportStateVerify:        true,
			},
			{
				ProtoV5ProviderFactories: testProtoV5Factories,
				Config:                   accProvider() + config,
				ResourceName:             "portainer_team_membership.m",
				ImportState:              true,
				ImportStateVerify:        true,
			},
		},
	})
}

// fakeDenyTeams is GET/PUT/POST/DELETE /teams with DenyPortainerAccess,
// which only the Business Edition stores (ignore = Community Edition).
type fakeDenyTeams struct {
	mu     sync.Mutex
	deny   *bool
	exists bool
	ignore bool
}

func newFakeDenyTeams(m *MockServer, ignore bool) *fakeDenyTeams {
	f := &fakeDenyTeams{ignore: ignore}
	store := func(r *http.Request) {
		var body struct {
			DenyPortainerAccess *bool
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		f.mu.Lock()
		defer f.mu.Unlock()
		f.exists = true
		if body.DenyPortainerAccess != nil && !f.ignore {
			f.deny = body.DenyPortainerAccess
		}
	}
	team := func() map[string]interface{} {
		f.mu.Lock()
		defer f.mu.Unlock()
		return map[string]interface{}{"Id": 1, "Name": "devops", "DenyPortainerAccess": f.deny != nil && *f.deny}
	}
	m.On("GET", "/teams", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		exists := f.exists
		f.mu.Unlock()
		if !exists {
			RespondJSON(http.StatusOK, []interface{}{})(w, r)
			return
		}
		RespondJSON(http.StatusOK, []interface{}{team()})(w, r)
	})
	m.On("POST", "/teams", func(w http.ResponseWriter, r *http.Request) { store(r); RespondJSON(http.StatusOK, team())(w, r) })
	m.On("PUT", "/teams/1", func(w http.ResponseWriter, r *http.Request) { store(r); RespondJSON(http.StatusOK, team())(w, r) })
	m.On("GET", "/teams/1", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		exists := f.exists
		f.mu.Unlock()
		if !exists {
			RespondString(http.StatusNotFound, "", "not found")(w, r)
			return
		}
		RespondJSON(http.StatusOK, team())(w, r)
	})
	m.On("DELETE", "/teams/1", func(w http.ResponseWriter, _ *http.Request) {
		f.mu.Lock()
		f.exists = false
		f.mu.Unlock()
		w.WriteHeader(http.StatusNoContent)
	})
	return f
}

// TestTeamResource_DenyPortainerAccess: sent on create and update only when
// configured, read back while managed (drift shows), left alone otherwise.
func TestTeamResource_DenyPortainerAccess(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	fake := newFakeDenyTeams(mock, false)
	sent := func(method, path string, want interface{}) resource.TestCheckFunc {
		return func(*terraform.State) error {
			var last *RecordedRequest
			for _, r := range mock.Requests() {
				if r.Method == method && r.Path == path {
					last = r
				}
			}
			var body map[string]interface{}
			if last == nil || last.DecodeJSON(&body) != nil {
				return fmt.Errorf("expected a %s %s request", method, path)
			}
			if got, ok := body["DenyPortainerAccess"]; (want == nil && ok) || (want != nil && got != want) {
				return fmt.Errorf("%s %s DenyPortainerAccess: want %v, got %v (present=%v)", method, path, want, got, ok)
			}
			return nil
		}
	}
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{
				Config: mockProviderConfig(mock) + `resource "portainer_team" "t" {
  name                  = "devops"
  deny_portainer_access = true
}`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("portainer_team.t", "deny_portainer_access", "true"),
					sent("POST", "/teams", true),
				),
			},
			{
				PreConfig: func() {
					fake.mu.Lock()
					f := false
					fake.deny = &f
					fake.mu.Unlock()
				},
				Config: mockProviderConfig(mock) + `resource "portainer_team" "t" {
  name                  = "devops"
  deny_portainer_access = true
}`,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction("portainer_team.t", plancheck.ResourceActionUpdate)},
				},
				Check: sent("PUT", "/teams/1", true),
			},
			{
				Config: mockProviderConfig(mock) + `resource "portainer_team" "t" { name = "devops" }`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckNoResourceAttr("portainer_team.t", "deny_portainer_access"),
					sent("PUT", "/teams/1", nil),
				),
			},
		},
	})
}

// TestTeamResource_DenyPortainerAccessIgnored: a server that ignores the
// field (Community Edition) fails the apply with the requirement.
func TestTeamResource_DenyPortainerAccessIgnored(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	newFakeDenyTeams(mock, true)
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{{
			Config: mockProviderConfig(mock) + `resource "portainer_team" "t" {
  name                  = "devops"
  deny_portainer_access = true
}`,
			ExpectError: regexp.MustCompile(`did not apply deny_portainer_access = true`),
		}},
	})
}

// TestTeamResource_DenyPortainerAccess_UpgradeFromV300: a team whose members
// are denied access plans no change after upgrading, without the attribute.
func TestTeamResource_DenyPortainerAccess_UpgradeFromV300(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	fake := newFakeDenyTeams(mock, false)
	deny := true
	fake.deny = &deny
	config := mockProviderConfig(mock) + `resource "portainer_team" "t" { name = "devops" }`
	resource.Test(t, resource.TestCase{
		IsUnitTest: true,
		Steps: []resource.TestStep{
			{ExternalProviders: previousRelease, Config: config},
			{
				ProtoV5ProviderFactories: testProtoV5Factories,
				Config:                   config,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
		},
	})
}
