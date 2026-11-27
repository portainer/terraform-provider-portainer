package internal

import (
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// TestAlertingRulesDataSource_Filters: name and category narrow the list,
// which is ordered by ID and carries the rule attributes.
func TestAlertingRulesDataSource_Filters(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("GET", "/observability/alerting/rules", RespondJSON(http.StatusOK, []map[string]interface{}{
		{"id": 9, "name": "Node NotReady", "category": "environment", "isInternal": true, "metricType": "days"},
		{"id": 4, "name": "Node NotReady", "category": "environment", "tagIds": []int{3}, "labels": map[string]string{"team": "ops"}},
		{"id": 2, "name": "Node NotReady", "category": "security"},
		{"id": 1, "name": "Backup Failure", "category": "portainer"},
	}))

	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{{
			Config: mockProviderConfig(mock) + `
data "portainer_alerting_rules" "r" {
  name     = "Node NotReady"
  category = "environment"
}

data "portainer_alerting_rules" "all" {}`,
			Check: resource.ComposeTestCheckFunc(
				resource.TestCheckResourceAttr("data.portainer_alerting_rules.r", "id", "portainer-alerting-rules-environment-Node NotReady"),
				resource.TestCheckResourceAttr("data.portainer_alerting_rules.r", "rules.#", "2"),
				resource.TestCheckResourceAttr("data.portainer_alerting_rules.r", "rules.0.rule_id", "4"),
				resource.TestCheckResourceAttr("data.portainer_alerting_rules.r", "rules.1.rule_id", "9"),
				resource.TestCheckResourceAttr("data.portainer_alerting_rules.r", "rules.0.tag_ids.0", "3"),
				resource.TestCheckResourceAttr("data.portainer_alerting_rules.r", "rules.0.labels.team", "ops"),
				resource.TestCheckResourceAttr("data.portainer_alerting_rules.r", "rules.1.metric_type", "days"),
				resource.TestCheckResourceAttr("data.portainer_alerting_rules.r", "rules.1.is_internal", "true"),
				resource.TestCheckResourceAttr("data.portainer_alerting_rules.all", "id", "portainer-alerting-rules--"),
				resource.TestCheckResourceAttr("data.portainer_alerting_rules.all", "rules.#", "4"),
			),
		}},
	})
}

// TestAlertingRulesDataSource_Unsupported names the requirement on a server
// without the alerting API instead of failing with a bare 404.
func TestAlertingRulesDataSource_Unsupported(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("GET", "/observability/alerting/rules", RespondString(http.StatusNotFound, "text/plain", "404 page not found"))
	mock.On("GET", "/system/version", RespondJSON(http.StatusOK, map[string]interface{}{"ServerVersion": "2.45.3", "ServerEdition": "CE"}))

	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{{
			Config:      mockProviderConfig(mock) + `data "portainer_alerting_rules" "r" {}`,
			ExpectError: regexp.MustCompile(`requires Portainer\s+Business\s+Edition;`),
		}},
	})
}

// TestAlertingRuleEnvironmentsDataSource_ReadsDeliveryStatus covers the data
// source that answers what an attachment cannot: whether the rule is actually
// evaluated where it was attached.
func TestAlertingRuleEnvironmentsDataSource_ReadsDeliveryStatus(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("GET", "/observability/alerting/rules/7/environments", RespondJSON(http.StatusOK, map[string]interface{}{
		"total": 3, "active": 1, "pending": 1, "error": 1, "excluded": 0, "notSupported": 0,
		"groups": []map[string]interface{}{{
			"endpointGroupId": 2, "endpointGroupName": "shops", "size": 3,
			"environments": []map[string]interface{}{
				{"endpointId": 12, "name": "shop-3", "status": "active", "updatedAt": 1756000000},
				{"endpointId": 13, "name": "shop-4", "status": "error", "reasonCode": "agent_unreachable", "message": "agent did not respond", "updatedAt": 1756000100},
			},
		}},
	}))

	const ds = "data.portainer_alerting_rule_environments.e"
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{{
			Config: mockProviderConfig(mock) + `data "portainer_alerting_rule_environments" "e" { rule_id = 7 }`,
			Check: resource.ComposeTestCheckFunc(
				resource.TestCheckResourceAttr(ds, "id", "portainer-alerting-rule-environments-7"),
				resource.TestCheckResourceAttr(ds, "total", "3"),
				resource.TestCheckResourceAttr(ds, "error", "1"),
				resource.TestCheckResourceAttr(ds, "groups.#", "1"),
				resource.TestCheckResourceAttr(ds, "groups.0.endpoint_group_name", "shops"),
				resource.TestCheckResourceAttr(ds, "groups.0.size", "3"),
				resource.TestCheckResourceAttr(ds, "groups.0.environments.#", "2"),
				resource.TestCheckResourceAttr(ds, "groups.0.environments.1.status", "error"),
				resource.TestCheckResourceAttr(ds, "groups.0.environments.1.reason_code", "agent_unreachable"),
				resource.TestCheckResourceAttr(ds, "groups.0.environments.1.updated_at", "1756000100"),
			),
		}},
	})
}

func TestObservabilityEnvironmentDataSource_HappyPath(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("GET", "/observability/environments/3", RespondJSON(http.StatusOK, map[string]interface{}{
		"enabled": true, "provider": "oneuptime", "url": "https://oneuptime.example.com",
		"projectId": "p-1", "clusterId": "c-9", "clusterName": "prod",
	}))

	const ds = "data.portainer_observability_environment.o"
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{{
			Config: mockProviderConfig(mock) + `data "portainer_observability_environment" "o" { environment_id = 3 }`,
			Check: resource.ComposeTestCheckFunc(
				resource.TestCheckResourceAttr(ds, "id", "portainer-observability-environment-3"),
				resource.TestCheckResourceAttr(ds, "enabled", "true"),
				resource.TestCheckResourceAttr(ds, "provider_name", "oneuptime"),
				resource.TestCheckResourceAttr(ds, "url", "https://oneuptime.example.com"),
				resource.TestCheckResourceAttr(ds, "project_id", "p-1"),
				resource.TestCheckResourceAttr(ds, "cluster_id", "c-9"),
				resource.TestCheckResourceAttr(ds, "cluster_name", "prod"),
			),
		}},
	})
}

func TestObservabilityEnvironmentDataSource_OldServer(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("GET", "/system/version", RespondJSON(http.StatusOK, map[string]interface{}{"ServerVersion": "2.39.1", "ServerEdition": "BE"}))

	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{{
			Config:      mockProviderConfig(mock) + `data "portainer_observability_environment" "o" { environment_id = 3 }`,
			ExpectError: regexp.MustCompile(`requires Portainer\s+Business\s+Edition\s+2.45\s+or\s+later`),
		}},
	})
}

// TestAlertingConnectivityDataSource: an unreachable Alertmanager fails the
// plan by default, and is reported when fail_on_error is off.
func TestAlertingConnectivityDataSource(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("GET", "/observability/alerting/connectivity", RespondString(http.StatusBadGateway,
		"application/json", `{"message":"dial tcp: connection refused"}`))

	const ds = "data.portainer_alerting_connectivity.c"
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{
				Config:      mockProviderConfig(mock) + `data "portainer_alerting_connectivity" "c" { url = "http://alertmanager.example.com:9093" }`,
				ExpectError: regexp.MustCompile(`could not be reached\s+\(set fail_on_error = false`),
			},
			{
				Config: mockProviderConfig(mock) + `
data "portainer_alerting_connectivity" "c" {
  url           = "http://alertmanager.example.com:9093"
  fail_on_error = false
}`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(ds, "reachable", "false"),
					resource.TestCheckResourceAttr(ds, "details", ""),
					resource.TestCheckResourceAttr(ds, "id", "portainer-alerting-connectivity-http://alertmanager.example.com:9093"),
					resource.TestCheckResourceAttrWith(ds, "error", func(v string) error {
						if !strings.Contains(v, "connection refused") {
							return fmt.Errorf("the failure reason must be reported, got %q", v)
						}
						return nil
					}),
					func(*terraform.State) error {
						req := mock.FindRequest("GET", "/observability/alerting/connectivity")
						if req == nil || !strings.Contains(req.Query, "url=http") {
							return fmt.Errorf("expected the Alertmanager URL in the query")
						}
						return nil
					},
				),
			},
		},
	})
}

// TestAlertingConnectivityDataSource_DefaultsToFailOnError: an answered
// check reads back fail_on_error as true, the SDKv2 default.
func TestAlertingConnectivityDataSource_DefaultsToFailOnError(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("GET", "/observability/alerting/connectivity", RespondJSON(http.StatusOK, map[string]interface{}{"reachable": true}))

	const ds = "data.portainer_alerting_connectivity.c"
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{{
			Config: mockProviderConfig(mock) + `data "portainer_alerting_connectivity" "c" { url = "http://am:9093" }`,
			Check: resource.ComposeTestCheckFunc(
				resource.TestCheckResourceAttr(ds, "fail_on_error", "true"),
				resource.TestCheckResourceAttr(ds, "reachable", "true"),
				resource.TestCheckResourceAttr(ds, "error", ""),
				resource.TestCheckResourceAttr(ds, "details", "{\"reachable\":true}\n"),
			),
		}},
	})
}

// TestAccAlertingRulesDataSource lists the rules of a Business Edition
// instance, which predefines some.
func TestAccAlertingRulesDataSource(t *testing.T) {
	requireAlertingBE(t)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { accPreCheck(t) },
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{{
			Config: accProvider() + `data "portainer_alerting_rules" "all" {}`,
			Check:  resource.TestCheckResourceAttrSet("data.portainer_alerting_rules.all", "rules.0.rule_id"),
		}},
	})
}
