package internal

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

const egMembershipConfig = `resource "portainer_endpoint_group_membership" "m" {
  endpoint_group_id = 5
  endpoint_id       = 9
}`

// TestEndpointGroupMembershipResource_Lifecycle: the PUT places the
// environment in the group, the ID is "<group>/<environment>", and destroy
// sends the DELETE that moves it back to Unassigned.
func TestEndpointGroupMembershipResource_Lifecycle(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	fake := newFakeEndpointGroups(mock)
	fake.setEndpointGroup(9, 1)

	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{{
			Config: mockProviderConfig(mock) + egMembershipConfig,
			Check: resource.ComposeTestCheckFunc(
				resource.TestCheckResourceAttr("portainer_endpoint_group_membership.m", "id", "5/9"),
				func(*terraform.State) error {
					if mock.FindRequest("PUT", "/endpoint_groups/5/endpoints/9") == nil {
						return fmt.Errorf("expected PUT /endpoint_groups/5/endpoints/9")
					}
					return nil
				},
			),
		}},
		CheckDestroy: func(*terraform.State) error {
			if mock.FindRequest("DELETE", "/endpoint_groups/5/endpoints/9") == nil {
				return fmt.Errorf("expected DELETE /endpoint_groups/5/endpoints/9")
			}
			return nil
		},
	})
}

// TestEndpointGroupMembershipResource_Drift: an environment moved to another
// group, or deleted, ends the membership.
func TestEndpointGroupMembershipResource_Drift(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	fake := newFakeEndpointGroups(mock)
	fake.setEndpointGroup(9, 1)
	config := mockProviderConfig(mock) + egMembershipConfig

	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{Config: config},
			{
				PreConfig:          func() { fake.setEndpointGroup(9, 7) },
				Config:             config,
				PlanOnly:           true,
				ExpectNonEmptyPlan: true,
			},
			{Config: config},
			{
				PreConfig:          func() { fake.removeEndpoint(9) },
				Config:             config,
				PlanOnly:           true,
				ExpectNonEmptyPlan: true,
			},
		},
	})
}

// TestEndpointGroupMembershipResource_Replace: changing the group replaces
// the membership.
func TestEndpointGroupMembershipResource_Replace(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	fake := newFakeEndpointGroups(mock)
	fake.setEndpointGroup(9, 1)

	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{Config: mockProviderConfig(mock) + egMembershipConfig},
			{
				Config: mockProviderConfig(mock) + `resource "portainer_endpoint_group_membership" "m" {
  endpoint_group_id = 6
  endpoint_id       = 9
}`,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction("portainer_endpoint_group_membership.m", plancheck.ResourceActionDestroyBeforeCreate)},
				},
				Check: resource.TestCheckResourceAttr("portainer_endpoint_group_membership.m", "id", "6/9"),
			},
		},
	})
}

func TestEndpointGroupMembershipResource_UpgradeFromV300(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	newFakeEndpointGroups(mock).setEndpointGroup(9, 1)
	config := mockProviderConfig(mock) + egMembershipConfig
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
