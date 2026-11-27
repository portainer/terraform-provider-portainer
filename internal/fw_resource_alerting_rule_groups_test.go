package internal

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// Alert rule assignment (Business Edition): which environments a rule covers.
// The endpoint applies to Portainer's built-in rules as much as to rules
// Terraform created, which is why it is a resource of its own.

// TestAlertingRuleGroupsResource_Lifecycle pins the payload, that the
// attachments are read back from the rule itself (the attachment endpoint
// only accepts writes), that an empty list is sent rather than omitted, and
// that destroy detaches every group but keeps the rule.
func TestAlertingRuleGroupsResource_Lifecycle(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	fake := newFakeAlertingRules(map[string]interface{}{"id": 7, "name": "High CPU", "isInternal": true})
	fake.register(mock)

	config := func(ids string) string {
		return mockProviderConfig(mock) + fmt.Sprintf(`
resource "portainer_alerting_rule_groups" "g" {
  rule_id            = 7
  endpoint_group_ids = %s
}`, ids)
	}
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		CheckDestroy: func(*terraform.State) error {
			body := lastAlertingBody(t, mock, "PUT", "/observability/alerting/rules/7/groups")
			if ids, ok := body["endpointGroupIds"].([]interface{}); !ok || len(ids) != 0 {
				return fmt.Errorf("destroy must detach every group, got %v", body["endpointGroupIds"])
			}
			if countAlertingRequests(mock, "DELETE", "/observability/alerting/rules/7") != 0 {
				return fmt.Errorf("destroy must not delete the rule itself")
			}
			return nil
		},
		Steps: []resource.TestStep{
			{
				Config: config("[2, 5]"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("portainer_alerting_rule_groups.g", "id", "7"),
					resource.TestCheckResourceAttr("portainer_alerting_rule_groups.g", "endpoint_group_ids.#", "2"),
					resource.TestCheckResourceAttr("portainer_alerting_rule_groups.g", "endpoint_group_ids.0", "2"),
					func(*terraform.State) error {
						ids, _ := lastAlertingBody(t, mock, "PUT", "/observability/alerting/rules/7/groups")["endpointGroupIds"].([]interface{})
						if len(ids) != 2 || ids[0] != float64(2) || ids[1] != float64(5) {
							return fmt.Errorf("expected the configured groups in order, got %v", ids)
						}
						return nil
					},
				),
			},
			{
				Config: config("[]"),
				Check: func(*terraform.State) error {
					ids, ok := lastAlertingBody(t, mock, "PUT", "/observability/alerting/rules/7/groups")["endpointGroupIds"].([]interface{})
					if !ok || len(ids) != 0 {
						return fmt.Errorf("expected an explicit empty list, got %v", ids)
					}
					return nil
				},
			},
			{
				ResourceName:      "portainer_alerting_rule_groups.g",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

// TestAlertingRuleGroupsResource_MissingRule: a rule deleted outside
// Terraform takes its attachment resource out of state.
func TestAlertingRuleGroupsResource_MissingRule(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	fake := newFakeAlertingRules(map[string]interface{}{"id": 7})
	fake.register(mock)

	config := mockProviderConfig(mock) + `
resource "portainer_alerting_rule_groups" "g" {
  rule_id            = 7
  endpoint_group_ids = [1]
}`
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{Config: config},
			{
				PreConfig:          func() { fake.remove(7) },
				Config:             config,
				PlanOnly:           true,
				ExpectNonEmptyPlan: true,
			},
		},
	})
}

// TestAlertingRuleGroups_UpgradeFromV300: state written by v3.0.0 (SDKv2)
// plans no change with this build, for a list and an empty list.
func TestAlertingRuleGroups_UpgradeFromV300(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	fake := newFakeAlertingRules(map[string]interface{}{"id": 7}, map[string]interface{}{"id": 8})
	fake.register(mock)

	config := mockProviderConfig(mock) + `
resource "portainer_alerting_rule_groups" "some" {
  rule_id            = 7
  endpoint_group_ids = [3, 1]
}

resource "portainer_alerting_rule_groups" "none" {
  rule_id            = 8
  endpoint_group_ids = []
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
