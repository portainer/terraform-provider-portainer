package internal

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// TestAlertingRuleTiersResource_SendsOrderedLadder pins the tier payload. The
// tiers are a list rather than a set because they are an ordered ladder of
// severities and Portainer stores them in the order given. Destroy does not
// call Portainer: there is no endpoint to clear a tier configuration.
func TestAlertingRuleTiersResource_SendsOrderedLadder(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	fake := newFakeAlertingRules(map[string]interface{}{"id": 7, "description": "CPU saturation", "enabled": false})
	fake.register(mock)

	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		CheckDestroy: func(*terraform.State) error {
			for _, r := range mock.Requests() {
				if r.Method != http.MethodGet && r.Path != "/observability/alerting/rules/7/tiers" {
					return fmt.Errorf("destroy must not call Portainer, got %s %s", r.Method, r.Path)
				}
			}
			return nil
		},
		Steps: []resource.TestStep{
			{
				Config: mockProviderConfig(mock) + `
resource "portainer_alerting_rule_tiers" "t" {
  rule_id            = 7
  condition_operator = ">"
  duration           = 5
  tier {
    severity  = "critical"
    threshold = 90
  }
  tier {
    severity  = "warning"
    threshold = 75
    enabled   = false
  }
}`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("portainer_alerting_rule_tiers.t", "id", "7"),
					resource.TestCheckResourceAttr("portainer_alerting_rule_tiers.t", "use_tiers", "true"),
					resource.TestCheckResourceAttr("portainer_alerting_rule_tiers.t", "enabled", "true"),
					resource.TestCheckResourceAttr("portainer_alerting_rule_tiers.t", "description", "CPU saturation"),
					func(*terraform.State) error {
						body := lastAlertingBody(t, mock, "PUT", "/observability/alerting/rules/7/tiers")
						if body["enabled"] != true || body["conditionOperator"] != ">" || body["duration"] != float64(5) {
							return fmt.Errorf("the scalar fields were not sent: %v", body)
						}
						if _, sent := body["description"]; sent {
							return fmt.Errorf("an unset description must not be sent on the first write")
						}
						tiers, _ := body["tiers"].([]interface{})
						if len(tiers) != 2 {
							return fmt.Errorf("expected two tiers, got %v", body["tiers"])
						}
						first, second := tiers[0].(map[string]interface{}), tiers[1].(map[string]interface{})
						if first["severity"] != "critical" || first["threshold"] != float64(90) || first["enabled"] != true {
							return fmt.Errorf("the ladder order was not preserved: %v", tiers)
						}
						if second["enabled"] != false {
							return fmt.Errorf("tier enabled = false was not sent: %v", second)
						}
						return nil
					},
				),
			},
			{
				ResourceName:      "portainer_alerting_rule_tiers.t",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

// TestAlertingRuleTiersResource_OmitsUnknownOptionals covers the first write
// against a built-in rule: the fields the configuration never set must not
// reach the payload, because Portainer keeps its own value for an omitted
// field.
func TestAlertingRuleTiersResource_OmitsUnknownOptionals(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	fake := newFakeAlertingRules(map[string]interface{}{"id": 7})
	fake.register(mock)

	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{{
			Config: mockProviderConfig(mock) + `
resource "portainer_alerting_rule_tiers" "t" {
  rule_id = 7
  tier {
    severity  = "warning"
    threshold = 75
  }
}`,
			Check: func(*terraform.State) error {
				body := lastAlertingBody(t, mock, "PUT", "/observability/alerting/rules/7/tiers")
				for _, key := range []string{"description", "conditionOperator", "duration"} {
					if _, ok := body[key]; ok {
						return fmt.Errorf("%s was not configured, so it must not be sent: %v", key, body[key])
					}
				}
				if _, ok := body["tiers"]; !ok {
					return fmt.Errorf("the tiers themselves must always be sent")
				}
				return nil
			},
		}},
	})
}

// TestAlertingRuleTiers_UpgradeFromV300: state written by v3.0.0 (SDKv2)
// plans no change with this build, with and without the optional fields.
func TestAlertingRuleTiers_UpgradeFromV300(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	fake := newFakeAlertingRules(
		map[string]interface{}{"id": 7, "description": "built-in", "conditionOperator": ">", "duration": 3},
		map[string]interface{}{"id": 8},
	)
	fake.register(mock)

	config := mockProviderConfig(mock) + `
resource "portainer_alerting_rule_tiers" "minimal" {
  rule_id = 7
  tier {
    severity  = "warning"
    threshold = 75
  }
}

resource "portainer_alerting_rule_tiers" "full" {
  rule_id            = 8
  enabled            = false
  description        = "tuned"
  condition_operator = ">="
  duration           = 10
  tier {
    severity  = "critical"
    threshold = 92.5
    enabled   = false
  }
  tier {
    severity  = "info"
    threshold = 50
  }
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
