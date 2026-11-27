package internal

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// portainer_alerting_rule talks to /observability/alerting/rules:
//   - with `rule_id` it adopts an existing (predefined) rule: it GETs the
//     rule, PUTs it back with the configuration laid over it and reads it
//     back;
//   - without `rule_id` it POSTs a custom rule, takes the ID from the
//     response (or, failing that, from the list by name) and marks the
//     resource `created`;
//   - an update is the same GET + overlay + PUT as an adoption;
//   - destroy sends DELETE only for a `created` rule.

// lastAlertingBody decodes the body of the last method+path request.
func lastAlertingBody(t *testing.T, mock *MockServer, method, path string) map[string]interface{} {
	t.Helper()
	var last *RecordedRequest
	for _, r := range mock.Requests() {
		if r.Method == method && r.Path == path {
			last = r
		}
	}
	if last == nil {
		t.Fatalf("expected a %s %s request", method, path)
	}
	out := map[string]interface{}{}
	if err := json.Unmarshal(last.Body, &out); err != nil {
		t.Fatalf("%s %s body is not JSON: %v", method, path, err)
	}
	return out
}

func countAlertingRequests(mock *MockServer, method, path string) int {
	n := 0
	for _, r := range mock.Requests() {
		if r.Method == method && r.Path == path {
			n++
		}
	}
	return n
}

func predefinedAlertingRule(id int) map[string]interface{} {
	return map[string]interface{}{
		"id": id, "name": "High CPU", "description": "CPU usage high", "enabled": false,
		"severity": "warning", "metricType": "percentage", "conditionOperator": ">", "threshold": 80,
		"duration": 120, "isInternal": true, "isEditable": true, "category": "environment",
		"promqlTemplate": "kube_cpu{{.X}}", "endpointGroupIds": []int{7},
		"tiers": []map[string]interface{}{
			{"severity": "critical", "threshold": 95, "enabled": true, "selector": `mountpoint="/"`},
		},
	}
}

// TestAlertingRuleResource_Adopt adopts a predefined rule: the PUT carries the
// rule as Portainer returned it (template, tiers) overlaid with what the
// configuration sets, never a tier selector; the rule is not created, so
// destroying the resource leaves it in Portainer.
func TestAlertingRuleResource_Adopt(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	fake := newFakeAlertingRules(predefinedAlertingRule(42))
	fake.register(mock)

	config := mockProviderConfig(mock) + `
resource "portainer_alerting_rule" "r" {
  rule_id   = 42
  enabled   = true
  severity  = "critical"
  threshold = 90
}`
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		CheckDestroy: func(*terraform.State) error {
			if countAlertingRequests(mock, "DELETE", "/observability/alerting/rules/42") != 0 {
				return fmt.Errorf("an adopted rule must not be deleted")
			}
			if fake.get(42) == nil {
				return fmt.Errorf("the predefined rule is gone")
			}
			return nil
		},
		Steps: []resource.TestStep{
			{
				Config: config,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("portainer_alerting_rule.r", "id", "42"),
					resource.TestCheckResourceAttr("portainer_alerting_rule.r", "created", "false"),
					resource.TestCheckResourceAttr("portainer_alerting_rule.r", "name", "High CPU"),
					// The selector is never sent back, so the stored rule lost it.
					resource.TestCheckResourceAttr("portainer_alerting_rule.r", "tiers.0.severity", "critical"),
					resource.TestCheckResourceAttr("portainer_alerting_rule.r", "endpoint_group_ids.0", "7"),
					func(*terraform.State) error {
						rule, _ := lastAlertingBody(t, mock, "PUT", "/observability/alerting/rules/42")["AlertingRule"].(map[string]interface{})
						for key, want := range map[string]interface{}{
							"id": float64(42), "enabled": true, "severity": "critical", "threshold": float64(90),
							"promqlTemplate": "kube_cpu{{.X}}", "description": "CPU usage high",
						} {
							if rule[key] != want {
								return fmt.Errorf("AlertingRule.%s: expected %v, got %v", key, want, rule[key])
							}
						}
						tiers, _ := rule["tiers"].([]interface{})
						if len(tiers) != 1 {
							return fmt.Errorf("expected the tier to be sent back, got %v", rule["tiers"])
						}
						if _, sent := tiers[0].(map[string]interface{})["selector"]; sent {
							return fmt.Errorf("a tier's selector must not be sent")
						}
						return nil
					},
				),
			},
			{
				ResourceName:      "portainer_alerting_rule.r",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

// TestAlertingRuleResource_UpdateOverlaysConfig pins the read-modify-write
// update: values the configuration does not set (endpoint_group_ids here) are
// Portainer's, not the state's, and a false/zero the configuration does set
// is sent.
func TestAlertingRuleResource_UpdateOverlaysConfig(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	fake := newFakeAlertingRules(predefinedAlertingRule(8))
	fake.register(mock)

	step := func(enabled bool, threshold int) string {
		return mockProviderConfig(mock) + fmt.Sprintf(`
resource "portainer_alerting_rule" "r" {
  rule_id     = 8
  enabled     = %v
  threshold   = %d
  promql_expr = "up == 0"
}`, enabled, threshold)
	}
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{Config: step(true, 5)},
			{
				PreConfig: func() {
					// Somebody re-attaches the rule in the UI.
					fake.mu.Lock()
					fake.rules[8]["endpointGroupIds"] = []interface{}{float64(4)}
					fake.mu.Unlock()
				},
				Config: step(false, 0),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction("portainer_alerting_rule.r", plancheck.ResourceActionUpdate)},
				},
				Check: func(*terraform.State) error {
					rule, _ := lastAlertingBody(t, mock, "PUT", "/observability/alerting/rules/8")["AlertingRule"].(map[string]interface{})
					for key, want := range map[string]interface{}{
						"id": float64(8), "enabled": false, "threshold": float64(0), "promqlExpr": "up == 0",
						"promqlTemplate": "kube_cpu{{.X}}",
					} {
						if rule[key] != want {
							return fmt.Errorf("AlertingRule.%s: expected %v, got %v", key, want, rule[key])
						}
					}
					if groups, _ := rule["endpointGroupIds"].([]interface{}); len(groups) != 1 || groups[0] != float64(4) {
						return fmt.Errorf("endpointGroupIds: expected Portainer's [4], got %v", rule["endpointGroupIds"])
					}
					if _, sent := rule["severity"]; !sent {
						return fmt.Errorf("the rule's own severity must be sent back")
					}
					return nil
				},
			},
		},
	})
}

// TestAlertingRuleResource_CustomRule creates a rule with POST: the body
// carries the configured fields in the AlertingRule envelope and no id, the
// resource is marked created, and destroy deletes it.
func TestAlertingRuleResource_CustomRule(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	fake := newFakeAlertingRules()
	fake.register(mock)

	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		CheckDestroy: func(*terraform.State) error {
			if countAlertingRequests(mock, "DELETE", "/observability/alerting/rules/101") != 1 {
				return fmt.Errorf("a created rule must be deleted on destroy")
			}
			return nil
		},
		Steps: []resource.TestStep{
			{
				Config: mockProviderConfig(mock) + `
resource "portainer_alerting_rule" "r" {
  name                        = "Disk almost full"
  enabled                     = true
  category                    = "environment"
  promql_expr                 = "disk_used_percent"
  for_duration_minutes        = 10
  endpoint_group_ids          = [2]
  supported_environment_types = "kubernetes"
  labels                      = { team = "ops" }
}`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("portainer_alerting_rule.r", "id", "101"),
					resource.TestCheckResourceAttr("portainer_alerting_rule.r", "rule_id", "101"),
					resource.TestCheckResourceAttr("portainer_alerting_rule.r", "created", "true"),
					resource.TestCheckResourceAttr("portainer_alerting_rule.r", "labels.team", "ops"),
					func(*terraform.State) error {
						rule, ok := lastAlertingBody(t, mock, "POST", "/observability/alerting/rules")["AlertingRule"].(map[string]interface{})
						if !ok {
							return fmt.Errorf("expected the POST body to wrap the rule in an AlertingRule envelope")
						}
						for key, want := range map[string]interface{}{
							"name": "Disk almost full", "category": "environment", "promqlExpr": "disk_used_percent",
							"forDurationMinutes": float64(10), "supportedEnvironmentTypes": "kubernetes", "enabled": true,
						} {
							if rule[key] != want {
								return fmt.Errorf("AlertingRule.%s: expected %v, got %v", key, want, rule[key])
							}
						}
						if groups, _ := rule["endpointGroupIds"].([]interface{}); len(groups) != 1 || groups[0] != float64(2) {
							return fmt.Errorf("AlertingRule.endpointGroupIds: expected [2], got %v", rule["endpointGroupIds"])
						}
						if _, sent := rule["id"]; sent {
							return fmt.Errorf("a create must not send an id")
						}
						if _, sent := rule["threshold"]; sent {
							return fmt.Errorf("an unconfigured attribute must not be sent")
						}
						return nil
					},
				),
			},
			{
				ResourceName:            "portainer_alerting_rule.r",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"created"},
			},
		},
	})
}

// TestAlertingRuleResource_CustomRuleIDFromList covers a create response that
// carries no ID: the rule is found in the list by name, the newest custom
// match winning over an older namesake and a built-in rule.
func TestAlertingRuleResource_CustomRuleIDFromList(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	fake := newFakeAlertingRules(
		map[string]interface{}{"id": 3, "name": "Queue backlog", "isInternal": true},
		map[string]interface{}{"id": 12, "name": "Queue backlog"},
	)
	fake.postWithoutID = true
	fake.register(mock)

	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{{
			Config: mockProviderConfig(mock) + `
resource "portainer_alerting_rule" "r" {
  name    = "Queue backlog"
  enabled = true
}`,
			Check: resource.TestCheckResourceAttr("portainer_alerting_rule.r", "id", "101"),
		}},
	})
}

// TestAlertingRuleResource_CustomRuleNeedsName rejects a custom rule without
// a name before anything is sent.
func TestAlertingRuleResource_CustomRuleNeedsName(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{{
			Config:      mockProviderConfig(mock) + `resource "portainer_alerting_rule" "r" { enabled = true }`,
			ExpectError: regexp.MustCompile(`name is required to create a custom alert\s+rule`),
		}},
	})
	if n := len(mock.Requests()); n != 0 {
		t.Errorf("expected no request, got %d", n)
	}
}

// TestAlertingRuleResource_HTTPError surfaces a rejected PUT.
func TestAlertingRuleResource_HTTPError(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("GET", "/observability/alerting/rules/1", RespondJSON(http.StatusOK, map[string]interface{}{"id": 1}))
	mock.On("PUT", "/observability/alerting/rules/1", RespondString(http.StatusBadRequest, "application/json", `{"message":"invalid rule payload"}`))

	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{{
			Config: mockProviderConfig(mock) + `
resource "portainer_alerting_rule" "r" {
  rule_id = 1
  enabled = true
}`,
			ExpectError: regexp.MustCompile(`failed to update alert\s+rule\s+1`),
		}},
	})
}

// TestAlertingRuleResource_GoneOutsideTerraform: a rule deleted in Portainer
// is planned again.
func TestAlertingRuleResource_GoneOutsideTerraform(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	fake := newFakeAlertingRules()
	fake.register(mock)

	config := mockProviderConfig(mock) + `
resource "portainer_alerting_rule" "r" {
  name    = "x"
  enabled = true
}`
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{Config: config},
			{
				PreConfig:          func() { fake.remove(101) },
				Config:             config,
				PlanOnly:           true,
				ExpectNonEmptyPlan: true,
			},
		},
	})
}

// TestAlertingRule_UpgradeFromV300: rules written by v3.0.0 (SDKv2) - an
// adopted one setting few attributes, a custom one setting many - plan no
// change with this build.
func TestAlertingRule_UpgradeFromV300(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	fake := newFakeAlertingRules(predefinedAlertingRule(42))
	fake.register(mock)

	config := mockProviderConfig(mock) + `
resource "portainer_alerting_rule" "adopted" {
  rule_id = 42
  enabled = true
}

resource "portainer_alerting_rule" "custom" {
  name               = "Disk almost full"
  enabled            = true
  severity           = "warning"
  threshold          = 85.5
  promql_expr        = "disk_used_percent"
  endpoint_group_ids = []
  labels             = {}
}

resource "portainer_alerting_rule" "labelled" {
  name     = "Labelled"
  enabled  = false
  tag_ids  = [1, 2]
  labels   = { team = "ops" }
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

// TestAccAlertingRule_Adopt adopts one of Portainer's predefined rules, found
// with portainer_alerting_rules, on a Business Edition instance.
func TestAccAlertingRule_Adopt(t *testing.T) {
	requireAlertingBE(t)
	config := accProvider() + `
data "portainer_alerting_rules" "all" {}

resource "portainer_alerting_rule" "r" {
  rule_id = data.portainer_alerting_rules.all.rules[0].rule_id
  enabled = data.portainer_alerting_rules.all.rules[0].enabled
}`
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { accPreCheck(t) },
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{
				Config: config,
				Check:  resource.TestCheckResourceAttr("portainer_alerting_rule.r", "created", "false"),
			},
		},
	})
}

// TestAlertingRuleResource_AdoptThresholdRebuildsExpr: Portainer rebuilds a
// built-in rule's promql_expr from its threshold, so a threshold change plans
// the expression unknown instead of keeping the old one (which failed the
// apply with "Provider produced inconsistent result").
func TestAlertingRuleResource_AdoptThresholdRebuildsExpr(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	seed := predefinedAlertingRule(42)
	seed["promqlExpr"] = "increase(auth_failures[5m]) > 80"
	fake := newFakeAlertingRules(seed)
	fake.renderExpr = true
	fake.register(mock)

	config := func(threshold int) string {
		return mockProviderConfig(mock) + fmt.Sprintf(`
resource "portainer_alerting_rule" "r" {
  rule_id   = 42
  enabled   = true
  threshold = %d
}`, threshold)
	}
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{
				Config: config(85),
				Check:  resource.TestCheckResourceAttr("portainer_alerting_rule.r", "promql_expr", "increase(auth_failures[5m]) > 85"),
			},
			{
				Config: config(77),
				Check:  resource.TestCheckResourceAttr("portainer_alerting_rule.r", "promql_expr", "increase(auth_failures[5m]) > 77"),
			},
		},
	})
}
