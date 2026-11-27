package internal

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// TestUserMembershipsSyncResource_PostsToUser: the sync runs once, its ID
// names the user and the run, and a changed trigger runs it again.
func TestUserMembershipsSyncResource_PostsToUser(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	newFakeUsersAPI(mock)
	cfg := func(trigger string) string {
		return mockProviderConfig(mock) + fmt.Sprintf(`resource "portainer_user_memberships_sync" "s" {
  user_id  = 7
  triggers = { run = %q }
}`, trigger)
	}
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{
				Config: cfg("1"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestMatchResourceAttr("portainer_user_memberships_sync.s", "id", regexp.MustCompile(`^7-memberships-sync-\d+$`)),
					func(*terraform.State) error {
						if mock.FindRequest("POST", "/users/7/memberships/sync") == nil {
							return fmt.Errorf("no sync request was sent")
						}
						return nil
					},
				),
			},
			{
				Config: cfg("1"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
			{
				Config: cfg("2"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction("portainer_user_memberships_sync.s", plancheck.ResourceActionReplace)},
				},
				Check: func(*terraform.State) error {
					if n := len(requestsTo(mock, "POST", "/users/7/memberships/sync")); n != 2 {
						return fmt.Errorf("expected two syncs, got %d", n)
					}
					return nil
				},
			},
		},
	})
}

func TestUserMembershipsSyncResource_Error(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{{
			Config:      mockProviderConfig(mock) + `resource "portainer_user_memberships_sync" "s" { user_id = 7 }`,
			ExpectError: regexp.MustCompile(`failed to synchronise the team memberships of user 7`),
		}},
	})
}

func TestUserMembershipsSyncResource_UpgradeFromV300(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	newFakeUsersAPI(mock)
	config := mockProviderConfig(mock) + `
resource "portainer_user_memberships_sync" "plain" { user_id = 7 }

resource "portainer_user_memberships_sync" "triggered" {
  user_id  = 8
  triggers = { run = "1" }
}`
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
