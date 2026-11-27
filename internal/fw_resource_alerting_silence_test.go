package internal

import (
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

const alertingSilenceConfig = `
resource "portainer_alerting_silence" "s" {
  alert_manager_url = "http://am.example:9093"
  comment           = "maintenance window"
  created_by        = "ci"
  starts_at         = "2026-01-01T00:00:00Z"
  ends_at           = "2026-01-02T00:00:00Z"
  matchers {
    name     = "alertname"
    value    = "HighCPU"
    is_regex = false
  }
  matchers {
    name     = "severity"
    value    = "crit.*"
    is_regex = true
    is_equal = false
  }
}`

// TestAlertingSilenceResource_Lifecycle: the POST carries the silence and the
// AlertManager URL, the ID comes from `silenceID`, and destroy deletes the
// silence naming the AlertManager in the query.
func TestAlertingSilenceResource_Lifecycle(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("POST", "/observability/alerting/silence", RespondJSON(http.StatusOK, map[string]interface{}{"silenceID": "abc-123"}))
	mock.On("GET", "/observability/alerting/alerts", RespondJSON(http.StatusOK, []map[string]interface{}{}))
	mock.On("DELETE", "/observability/alerting/silence/abc-123", RespondString(http.StatusNoContent, "", ""))

	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		CheckDestroy: func(*terraform.State) error {
			del := mock.FindRequest("DELETE", "/observability/alerting/silence/abc-123")
			if del == nil || !strings.Contains(del.Query, "alertManagerURL=") {
				return fmt.Errorf("expected a DELETE naming the AlertManager, got %v", del)
			}
			return nil
		},
		Steps: []resource.TestStep{{
			Config: mockProviderConfig(mock) + alertingSilenceConfig,
			Check: resource.ComposeTestCheckFunc(
				resource.TestCheckResourceAttr("portainer_alerting_silence.s", "id", "abc-123"),
				resource.TestCheckResourceAttr("portainer_alerting_silence.s", "matchers.0.is_equal", "true"),
				func(*terraform.State) error {
					payload := lastAlertingBody(t, mock, "POST", "/observability/alerting/silence")
					if payload["alertManagerURL"] != "http://am.example:9093" {
						return fmt.Errorf("alertManagerURL: got %v", payload["alertManagerURL"])
					}
					silence, _ := payload["silence"].(map[string]interface{})
					if silence["comment"] != "maintenance window" || silence["createdBy"] != "ci" {
						return fmt.Errorf("unexpected silence %v", silence)
					}
					matchers, _ := silence["matchers"].([]interface{})
					if len(matchers) != 2 {
						return fmt.Errorf("expected two matchers, got %v", silence["matchers"])
					}
					m := matchers[0].(map[string]interface{})
					if m["name"] != "alertname" || m["value"] != "HighCPU" || m["isRegex"] != false || m["isEqual"] != true {
						return fmt.Errorf("unexpected matcher %v", m)
					}
					return nil
				},
			),
		}},
	})
}

// TestAlertingSilenceResource_IDKey takes the ID from `id` when the response
// has no `silenceID`, and tolerates a 404 on destroy.
func TestAlertingSilenceResource_IDKey(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("POST", "/observability/alerting/silence", RespondJSON(http.StatusOK, map[string]interface{}{"id": "xyz-9"}))
	mock.On("GET", "/observability/alerting/alerts", RespondJSON(http.StatusOK, []map[string]interface{}{}))
	mock.On("DELETE", "/observability/alerting/silence/xyz-9", RespondString(http.StatusNotFound, "application/json", `{"message":"silence not found"}`))

	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{{
			Config: mockProviderConfig(mock) + alertingSilenceConfig,
			Check:  resource.TestCheckResourceAttr("portainer_alerting_silence.s", "id", "xyz-9"),
		}},
	})
}

// TestAlertingSilenceResource_Errors: a rejected POST, and a response without
// an ID, fail the apply.
func TestAlertingSilenceResource_Errors(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct {
		handler http.HandlerFunc
		want    string
	}{
		"rejected": {RespondString(http.StatusBadRequest, "application/json", `{"message":"invalid silence"}`), `failed to create alert\s+silence`},
		"no id":    {RespondJSON(http.StatusOK, map[string]interface{}{}), `silence created but no ID\s+returned\s+in\s+response`},
	} {
		t.Run(name, func(t *testing.T) {
			mock := NewMockServer(t)
			mock.On("POST", "/observability/alerting/silence", tc.handler)
			resource.UnitTest(t, resource.TestCase{
				ProtoV5ProviderFactories: testProtoV5Factories,
				Steps: []resource.TestStep{{
					Config:      mockProviderConfig(mock) + alertingSilenceConfig,
					ExpectError: regexp.MustCompile(tc.want),
				}},
			})
		})
	}
}

// TestAlertingSilence_UpgradeFromV300: a silence written by v3.0.0 (SDKv2)
// plans no change with this build.
func TestAlertingSilence_UpgradeFromV300(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("POST", "/observability/alerting/silence", RespondJSON(http.StatusOK, map[string]interface{}{"silenceID": "abc-123"}))
	mock.On("GET", "/observability/alerting/alerts", RespondJSON(http.StatusOK, []map[string]interface{}{}))
	mock.On("DELETE", "/observability/alerting/silence/abc-123", RespondString(http.StatusNoContent, "", ""))

	config := mockProviderConfig(mock) + alertingSilenceConfig
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

// TestAccAlertingSilence_Basic silences an alert on a Business Edition
// instance with an AlertManager (PORTAINER_ACC_ALERTMANAGER_URL).
func TestAccAlertingSilence_Basic(t *testing.T) {
	requireAlertingBE(t)
	am := alertingEnvOrSkip(t, "PORTAINER_ACC_ALERTMANAGER_URL")
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { accPreCheck(t) },
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{{
			Config: accProvider() + strings.Replace(alertingSilenceConfig, "http://am.example:9093", am, 1),
			Check:  resource.TestCheckResourceAttrSet("portainer_alerting_silence.s", "id"),
		}},
	})
}
