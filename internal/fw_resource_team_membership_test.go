package internal

import (
	"fmt"
	"net/http"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// TestTeamMembershipResource_Lifecycle: create, a role change in place,
// import, destroy.
func TestTeamMembershipResource_Lifecycle(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	fake := newFakeUsersAPI(mock)
	user := fake.addUser("alice", "pw", 2)
	team := fake.addTeam("t")
	cfg := func(role int) string {
		return mockProviderConfig(mock) + fmt.Sprintf(`resource "portainer_team_membership" "m" {
  team_id = %d
  user_id = %d
  role    = %d
}`, team, user, role)
	}
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{
				Config: cfg(2),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("portainer_team_membership.m", "id", "3"),
					func(*terraform.State) error {
						var body map[string]interface{}
						if err := mock.FindRequest("POST", "/team_memberships").DecodeJSON(&body); err != nil ||
							body["role"] != float64(2) || body["teamID"] != float64(team) || body["userID"] != float64(user) {
							return fmt.Errorf("unexpected create payload %v (%w)", body, err)
						}
						return nil
					},
				),
			},
			{
				Config: cfg(1),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction("portainer_team_membership.m", plancheck.ResourceActionUpdate)},
				},
				Check: func(*terraform.State) error {
					var body map[string]interface{}
					if err := mock.FindRequest("PUT", "/team_memberships/3").DecodeJSON(&body); err != nil || body["role"] != float64(1) {
						return fmt.Errorf("unexpected update payload %v (%w)", body, err)
					}
					return nil
				},
			},
			{ResourceName: "portainer_team_membership.m", ImportState: true, ImportStateVerify: true},
		},
	})
	if len(fake.memberships) != 0 {
		t.Error("expected the membership deleted on destroy")
	}
}

// TestTeamMembershipResource_GoneOutsideTerraform: a membership missing from
// the list is planned again, and its destroy-time 404 is not an error.
func TestTeamMembershipResource_GoneOutsideTerraform(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	fake := newFakeUsersAPI(mock)
	config := mockProviderConfig(mock) + `resource "portainer_team_membership" "m" {
  team_id = 1
  user_id = 2
  role    = 2
}`
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{Config: config},
			{
				PreConfig: func() {
					fake.mu.Lock()
					delete(fake.memberships, 1)
					fake.mu.Unlock()
				},
				Config:             config,
				PlanOnly:           true,
				ExpectNonEmptyPlan: true,
			},
		},
	})
}

func TestTeamMembershipResource_ImportMissing(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	newFakeUsersAPI(mock)
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{{
			Config: mockProviderConfig(mock) + `resource "portainer_team_membership" "m" {
  team_id = 1
  user_id = 2
  role = 2
}`,
			ResourceName:  "portainer_team_membership.m",
			ImportState:   true,
			ImportStateId: "99",
			ExpectError:   regexp.MustCompile(`Cannot import non-existent remote object`),
		}},
	})
}

func TestTeamMembershipResource_Errors(t *testing.T) {
	t.Parallel()
	config := `resource "portainer_team_membership" "m" {
  team_id = 1
  user_id = 2
  role = 2
}`

	create := NewMockServer(t)
	create.On("POST", "/team_memberships", RespondJSON(http.StatusInternalServerError, map[string]string{"message": "boom"}))
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{{
			Config:      mockProviderConfig(create) + config,
			ExpectError: regexp.MustCompile(`failed to create team membership`),
		}},
	})

	list := NewMockServer(t)
	list.On("GET", "/team_memberships", RespondJSON(http.StatusInternalServerError, map[string]string{"message": "boom"}))
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{{
			Config:        mockProviderConfig(list) + config,
			ResourceName:  "portainer_team_membership.m",
			ImportState:   true,
			ImportStateId: "1",
			ExpectError:   regexp.MustCompile(`failed to fetch team memberships list`),
		}},
	})

	// Update and delete errors: the fake serves the create, then refuses.
	mock := NewMockServer(t)
	mock.On("PUT", "/team_memberships/1", RespondJSON(http.StatusInternalServerError, map[string]string{"message": "boom"}))
	fake := newFakeUsersAPI(mock)
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{Config: mockProviderConfig(mock) + config},
			{
				Config: mockProviderConfig(mock) + `resource "portainer_team_membership" "m" {
  team_id = 1
  user_id = 2
  role = 1
}`,
				ExpectError: regexp.MustCompile(`failed to update team membership`),
			},
		},
	})
	_ = fake
}

func TestTeamMembershipResource_UpgradeFromV300(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	fake := newFakeUsersAPI(mock)
	user := fake.addUser("alice", "pw", 2)
	team := fake.addTeam("t")
	config := mockProviderConfig(mock) + fmt.Sprintf(`resource "portainer_team_membership" "m" {
  team_id = %d
  user_id = %d
  role    = 1
}`, team, user)
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
