package internal

import (
	"fmt"
	"net/http"
	"os"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

func egPolicy(fake *fakeEndpointGroups, groupID int64, kind, principal string) (map[string]interface{}, bool) {
	fake.mu.Lock()
	defer fake.mu.Unlock()
	g := fake.groups[groupID]
	if g == nil {
		return nil, false
	}
	field := "TeamAccessPolicies"
	if kind == "user" {
		field = "UserAccessPolicies"
	}
	policies, _ := g[field].(map[string]interface{})
	p, ok := policies[principal].(map[string]interface{})
	return p, ok
}

// TestEndpointGroupAccessResource_TeamLifecycle: the team policy is merged
// into the whole group (name kept), the role is updated in place, both import
// ID forms work, and destroy removes the policy.
func TestEndpointGroupAccessResource_TeamLifecycle(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	fake := newFakeEndpointGroups(mock)
	fake.seed(4, "prod")

	cfg := func(role int) string {
		return mockProviderConfig(mock) + fmt.Sprintf(`resource "portainer_endpoint_group_access" "a" {
  endpoint_group_id = 4
  team_id           = 11
  role_id           = %d
}`, role)
	}
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{
				Config: cfg(3),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("portainer_endpoint_group_access.a", "id", "4:team:11"),
					resource.TestCheckNoResourceAttr("portainer_endpoint_group_access.a", "user_id"),
					func(*terraform.State) error {
						body := a1LastBody(mock, "PUT", "/endpoint_groups/4")
						if body["Name"] != "prod" {
							return fmt.Errorf("the PUT must carry the whole group, got %v", body)
						}
						teams, _ := body["TeamAccessPolicies"].(map[string]interface{})
						entry, _ := teams["11"].(map[string]interface{})
						if entry["RoleId"] != float64(3) {
							return fmt.Errorf("team 11 policy: %v", body["TeamAccessPolicies"])
						}
						return nil
					},
				),
			},
			{
				Config: cfg(2),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction("portainer_endpoint_group_access.a", plancheck.ResourceActionUpdate)},
				},
				Check: func(*terraform.State) error {
					if p, ok := egPolicy(fake, 4, "team", "11"); !ok || p["RoleId"] != float64(2) {
						return fmt.Errorf("role not updated: %v", p)
					}
					return nil
				},
			},
			{
				ResourceName:      "portainer_endpoint_group_access.a",
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				ResourceName:      "portainer_endpoint_group_access.a",
				ImportState:       true,
				ImportStateId:     "4/team/11",
				ImportStateVerify: true,
			},
		},
		CheckDestroy: func(*terraform.State) error {
			if _, ok := egPolicy(fake, 4, "team", "11"); ok {
				return fmt.Errorf("team 11 policy still present")
			}
			return nil
		},
	})
}

// TestEndpointGroupAccessResource_User: the user variant, with the default
// role 0 when role_id is omitted.
func TestEndpointGroupAccessResource_User(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	fake := newFakeEndpointGroups(mock)
	fake.seed(2, "dev")

	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{
				Config: mockProviderConfig(mock) + `resource "portainer_endpoint_group_access" "a" {
  endpoint_group_id = 2
  user_id           = 5
}`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("portainer_endpoint_group_access.a", "id", "2:user:5"),
					resource.TestCheckResourceAttr("portainer_endpoint_group_access.a", "role_id", "0"),
					func(*terraform.State) error {
						if _, ok := egPolicy(fake, 2, "user", "5"); !ok {
							return fmt.Errorf("user 5 policy missing")
						}
						return nil
					},
				),
			},
			{
				ResourceName:      "portainer_endpoint_group_access.a",
				ImportState:       true,
				ImportStateId:     "2:user:5",
				ImportStateVerify: true,
			},
		},
		CheckDestroy: func(*terraform.State) error {
			if _, ok := egPolicy(fake, 2, "user", "5"); ok {
				return fmt.Errorf("user 5 policy still present")
			}
			return nil
		},
	})
}

func TestEndpointGroupAccessResource_MissingTeamAndUser(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	newFakeEndpointGroups(mock).seed(4, "prod")
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{{
			Config:      mockProviderConfig(mock) + `resource "portainer_endpoint_group_access" "a" { endpoint_group_id = 4 }`,
			ExpectError: regexp.MustCompile(`either team_id or user_id must be provided`),
		}},
	})
}

// TestEndpointGroupAccessResource_Drift: a policy removed outside Terraform,
// or a deleted group, is planned for creation again.
func TestEndpointGroupAccessResource_Drift(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	fake := newFakeEndpointGroups(mock)
	fake.seed(4, "prod")
	config := mockProviderConfig(mock) + `resource "portainer_endpoint_group_access" "a" {
  endpoint_group_id = 4
  team_id           = 11
  role_id           = 3
}`
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{Config: config},
			{
				PreConfig: func() {
					fake.mu.Lock()
					fake.groups[4]["TeamAccessPolicies"] = map[string]interface{}{}
					fake.mu.Unlock()
				},
				Config:             config,
				PlanOnly:           true,
				ExpectNonEmptyPlan: true,
			},
			{
				PreConfig:          func() { fake.remove(4) },
				Config:             config,
				PlanOnly:           true,
				ExpectNonEmptyPlan: true,
			},
		},
	})
}

func TestEndpointGroupAccessResource_GroupNotFoundOnCreate(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	newFakeEndpointGroups(mock)
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{{
			Config: mockProviderConfig(mock) + `resource "portainer_endpoint_group_access" "a" {
  endpoint_group_id = 9
  team_id           = 1
}`,
			ExpectError: regexp.MustCompile(`endpoint group not found`),
		}},
	})
}

func TestEndpointGroupAccessResource_PUTError(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("GET", "/endpoint_groups/4", RespondJSON(http.StatusOK, map[string]interface{}{"Id": 4, "Name": "prod"}))
	mock.On("PUT", "/endpoint_groups/4", RespondString(http.StatusBadRequest, "application/json", `{"message":"bad"}`))
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{{
			Config: mockProviderConfig(mock) + `resource "portainer_endpoint_group_access" "a" {
  endpoint_group_id = 4
  team_id           = 1
}`,
			ExpectError: regexp.MustCompile(`failed to update endpoint group access`),
		}},
	})
}

func TestEndpointGroupAccessResource_ImportRejectsMalformedIDs(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	fake := newFakeEndpointGroups(mock)
	fake.seed(4, "prod")
	config := mockProviderConfig(mock) + `resource "portainer_endpoint_group_access" "a" {
  endpoint_group_id = 4
  team_id           = 11
}`
	ids := []string{"2:team:", "2:group:4", "x:team:4", "2:team:x", "2"}
	steps := make([]resource.TestStep, 0, len(ids)+1)
	steps = append(steps, resource.TestStep{Config: config})
	for _, id := range ids {
		steps = append(steps, resource.TestStep{
			Config:        config,
			ResourceName:  "portainer_endpoint_group_access.a",
			ImportState:   true,
			ImportStateId: id,
			ExpectError:   regexp.MustCompile(`invalid import ID`),
		})
	}
	resource.UnitTest(t, resource.TestCase{ProtoV5ProviderFactories: testProtoV5Factories, Steps: steps})
}

// TestEndpointGroupAccessResource_UpgradeFromV300: state written by v3.0.0
// plans no change with this build, for a team and a user policy.
func TestEndpointGroupAccessResource_UpgradeFromV300(t *testing.T) {
	t.Parallel()
	for name, cfg := range map[string]string{
		"team": `resource "portainer_endpoint_group_access" "a" {
  endpoint_group_id = 4
  team_id           = 11
  role_id           = 3
}`,
		"user_default_role": `resource "portainer_endpoint_group_access" "a" {
  endpoint_group_id = 4
  user_id           = 5
}`,
	} {
		t.Run(name, func(t *testing.T) {
			mock := NewMockServer(t)
			newFakeEndpointGroups(mock).seed(4, "prod")
			config := mockProviderConfig(mock) + cfg
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
		})
	}
}

// TestAccEndpointGroupAccessResource_basic grants a team access to a group
// and reads it back through the data source. Roles are a BE feature.
func TestAccEndpointGroupAccessResource_basic(t *testing.T) {
	if os.Getenv("PORTAINER_ACC_BE") != "1" {
		t.Skip("role-based access policies need Portainer BE: set PORTAINER_ACC_BE=1")
	}
	name := acctest.RandomWithPrefix("acc-ega")
	config := fmt.Sprintf(`
resource "portainer_endpoint_group" "g" { name = %[1]q }
resource "portainer_team" "t" { name = %[1]q }

resource "portainer_endpoint_group_access" "a" {
  endpoint_group_id = tonumber(portainer_endpoint_group.g.id)
  team_id           = tonumber(portainer_team.t.id)
  role_id           = 1
}

data "portainer_endpoint_group_access" "a" {
  endpoint_group_id = portainer_endpoint_group_access.a.endpoint_group_id
  team_id           = portainer_endpoint_group_access.a.team_id
}
`, name)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { accPreCheck(t) },
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{
				Config: accProvider() + config,
				Check:  resource.TestCheckResourceAttr("data.portainer_endpoint_group_access.a", "role_id", "1"),
			},
			{
				Config:            accProvider() + config,
				ResourceName:      "portainer_endpoint_group_access.a",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

// --- data source portainer_endpoint_group_access ---------------------------

func egAccessDSMock(t *testing.T) *MockServer {
	mock := NewMockServer(t)
	mock.On("GET", "/endpoint_groups/4", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id":                 4,
		"TeamAccessPolicies": map[string]interface{}{"11": map[string]int{"RoleId": 3}},
		"UserAccessPolicies": map[string]interface{}{"5": map[string]int{"RoleId": 1}},
	}))
	mock.On("GET", "/endpoint_groups/99", RespondString(http.StatusNotFound, "application/json", `{"message":"not found"}`))
	return mock
}

func TestEndpointGroupAccessDataSource_Read(t *testing.T) {
	t.Parallel()
	mock := egAccessDSMock(t)
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{{
			Config: mockProviderConfig(mock) + `
data "portainer_endpoint_group_access" "team" {
  endpoint_group_id = 4
  team_id           = 11
}
data "portainer_endpoint_group_access" "user" {
  endpoint_group_id = 4
  user_id           = 5
}`,
			Check: resource.ComposeTestCheckFunc(
				resource.TestCheckResourceAttr("data.portainer_endpoint_group_access.team", "id", "4/team/11"),
				resource.TestCheckResourceAttr("data.portainer_endpoint_group_access.team", "role_id", "3"),
				resource.TestCheckResourceAttr("data.portainer_endpoint_group_access.user", "id", "4/user/5"),
				resource.TestCheckResourceAttr("data.portainer_endpoint_group_access.user", "role_id", "1"),
			),
		}},
	})
}

func TestEndpointGroupAccessDataSource_Errors(t *testing.T) {
	t.Parallel()
	mock := egAccessDSMock(t)
	for name, tc := range map[string]struct{ config, want string }{
		"missing principal": {`data "portainer_endpoint_group_access" "a" { endpoint_group_id = 4 }`, `either team_id or user_id must be provided`},
		"policy missing":    {`data "portainer_endpoint_group_access" "a" {` + "\n" + `  endpoint_group_id = 4` + "\n" + `  team_id = 12` + "\n}", `access policy not found`},
		"group not found":   {`data "portainer_endpoint_group_access" "a" {` + "\n" + `  endpoint_group_id = 99` + "\n" + `  user_id = 5` + "\n}", `endpoint group 99 not found`},
	} {
		t.Run(name, func(t *testing.T) {
			resource.UnitTest(t, resource.TestCase{
				ProtoV5ProviderFactories: testProtoV5Factories,
				Steps: []resource.TestStep{{
					Config:      mockProviderConfig(mock) + tc.config,
					ExpectError: regexp.MustCompile(tc.want),
				}},
			})
		})
	}
}
