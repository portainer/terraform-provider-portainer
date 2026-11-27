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

// TestAlertingSettingsResource_Lifecycle: the PUT carries the configured
// settings in the AlertingSettings envelope, the ID comes from its response,
// an update PUTs again with that ID, and destroy disables alerting.
func TestAlertingSettingsResource_Lifecycle(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	fake := &fakeAlertingSettings{entries: []map[string]interface{}{{"id": float64(4), "enabled": false, "isInternal": true}}}
	fake.register(mock)

	config := func(name string) string {
		return mockProviderConfig(mock) + fmt.Sprintf(`
resource "portainer_alerting_settings" "s" {
  enabled       = true
  name          = %q
  url           = "http://am.example:9093"
  portainer_url = "http://portainer.example"
}`, name)
	}
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		CheckDestroy: func(*terraform.State) error {
			settings, _ := lastAlertingBody(t, mock, "PUT", "/observability/alerting/settings")["AlertingSettings"].(map[string]interface{})
			if got, present := settings["enabled"]; !present || got != false {
				return fmt.Errorf("destroy must send enabled=false, got %v (present=%v)", got, present)
			}
			if settings["id"] != float64(4) {
				return fmt.Errorf("destroy must name the entry, got %v", settings["id"])
			}
			return nil
		},
		Steps: []resource.TestStep{
			{
				Config: config("primary-am"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("portainer_alerting_settings.s", "id", "4"),
					resource.TestCheckResourceAttr("portainer_alerting_settings.s", "status", "connected"),
					resource.TestCheckResourceAttr("portainer_alerting_settings.s", "notification_channels.#", "0"),
					func(*terraform.State) error {
						settings, ok := lastAlertingBody(t, mock, "PUT", "/observability/alerting/settings")["AlertingSettings"].(map[string]interface{})
						if !ok {
							return fmt.Errorf("expected the AlertingSettings envelope")
						}
						if settings["enabled"] != true || settings["url"] != "http://am.example:9093" || settings["name"] != "primary-am" {
							return fmt.Errorf("unexpected payload %v", settings)
						}
						return nil
					},
				),
			},
			{
				Config: config("renamed"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("portainer_alerting_settings.s", "name", "renamed"),
					func(*terraform.State) error {
						settings, _ := lastAlertingBody(t, mock, "PUT", "/observability/alerting/settings")["AlertingSettings"].(map[string]interface{})
						if settings["id"] != float64(4) || settings["name"] != "renamed" {
							return fmt.Errorf("an update must PUT the entry by ID, got %v", settings)
						}
						return nil
					},
				),
			},
			{
				ResourceName:      "portainer_alerting_settings.s",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

// TestAlertingSettingsResource_NoIDFallback: a PUT response without an ID
// leaves the generic ID, which the read replaces with the first entry's.
func TestAlertingSettingsResource_NoIDFallback(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	fake := &fakeAlertingSettings{entries: []map[string]interface{}{{"id": float64(11), "enabled": true, "url": "http://am.example"}}, noIDInResponse: true}
	fake.register(mock)

	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{{
			Config: mockProviderConfig(mock) + `resource "portainer_alerting_settings" "s" { enabled = true }`,
			Check:  resource.TestCheckResourceAttr("portainer_alerting_settings.s", "id", "11"),
		}},
	})
}

// TestAlertingSettingsResource_SingleObjectResponse: a GET answering with one
// object rather than a list is read all the same.
func TestAlertingSettingsResource_SingleObjectResponse(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("PUT", "/observability/alerting/settings", RespondJSON(http.StatusOK, map[string]interface{}{"id": 8, "enabled": true}))
	mock.On("GET", "/observability/alerting/settings", RespondJSON(http.StatusOK, map[string]interface{}{
		"id": 8, "enabled": true, "name": "single-shape", "url": "http://am.example", "status": "disabled",
	}))

	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{{
			Config: mockProviderConfig(mock) + `resource "portainer_alerting_settings" "s" { enabled = true }`,
			Check: resource.ComposeTestCheckFunc(
				resource.TestCheckResourceAttr("portainer_alerting_settings.s", "id", "8"),
				resource.TestCheckResourceAttr("portainer_alerting_settings.s", "name", "single-shape"),
				resource.TestCheckResourceAttr("portainer_alerting_settings.s", "status", "disabled"),
			),
		}},
	})
}

// TestAlertingSettingsResource_HTTPErrors: a rejected PUT fails the apply and
// a failing GET fails the refresh.
func TestAlertingSettingsResource_HTTPErrors(t *testing.T) {
	t.Parallel()
	t.Run("put", func(t *testing.T) {
		mock := NewMockServer(t)
		mock.On("PUT", "/observability/alerting/settings", RespondString(http.StatusBadRequest, "application/json", `{"message":"bad payload"}`))
		resource.UnitTest(t, resource.TestCase{
			ProtoV5ProviderFactories: testProtoV5Factories,
			Steps: []resource.TestStep{{
				Config:      mockProviderConfig(mock) + `resource "portainer_alerting_settings" "s" { enabled = true }`,
				ExpectError: regexp.MustCompile(`(?s)failed to create/update alerting settings:.*bad payload`),
			}},
		})
	})
	t.Run("get", func(t *testing.T) {
		mock := NewMockServer(t)
		mock.On("PUT", "/observability/alerting/settings", RespondJSON(http.StatusOK, map[string]interface{}{"id": 5}))
		mock.On("GET", "/observability/alerting/settings", RespondString(http.StatusInternalServerError, "application/json", `{"message":"boom"}`))
		resource.UnitTest(t, resource.TestCase{
			ProtoV5ProviderFactories: testProtoV5Factories,
			Steps: []resource.TestStep{{
				Config:      mockProviderConfig(mock) + `resource "portainer_alerting_settings" "s" { enabled = true }`,
				ExpectError: regexp.MustCompile(`failed to read alerting settings,\s+status:\s+500`),
			}},
		})
	})
}

// TestAlertingSettingsResource_GoneOutsideTerraform: an empty list or a 404
// takes the settings out of state, so they are planned again.
func TestAlertingSettingsResource_GoneOutsideTerraform(t *testing.T) {
	t.Parallel()
	for name, gone := range map[string]http.HandlerFunc{
		"empty list": RespondJSON(http.StatusOK, []map[string]interface{}{}),
		"404":        RespondString(http.StatusNotFound, "application/json", `{"message":"not found"}`),
	} {
		t.Run(name, func(t *testing.T) {
			mock := NewMockServer(t)
			fake := &fakeAlertingSettings{}
			removed := false
			mock.On("GET", "/observability/alerting/settings", func(w http.ResponseWriter, r *http.Request) {
				if removed {
					gone(w, r)
					return
				}
				fake.mu.Lock()
				entries := fake.entries
				fake.mu.Unlock()
				RespondJSON(http.StatusOK, entries)(w, r)
			})
			fake.register(mock)

			config := mockProviderConfig(mock) + `resource "portainer_alerting_settings" "s" { enabled = true }`
			resource.UnitTest(t, resource.TestCase{
				ProtoV5ProviderFactories: testProtoV5Factories,
				Steps: []resource.TestStep{
					{Config: config},
					{
						PreConfig:          func() { removed = true },
						Config:             config,
						PlanOnly:           true,
						ExpectNonEmptyPlan: true,
					},
				},
			})
		})
	}
}

// TestAlertingSettingsResource_NotificationChannels: configured channels are
// sent, and the channels Portainer reports are read back into them.
func TestAlertingSettingsResource_NotificationChannels(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	fake := &fakeAlertingSettings{}
	fake.register(mock)

	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{
				Config: mockProviderConfig(mock) + `
resource "portainer_alerting_settings" "s" {
  enabled = true
  notification_channels {
    name    = "ops-slack"
    type    = "slack"
    enabled = true
    config  = { webhook_url = "https://hooks.slack.com/x" }
  }
}`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("portainer_alerting_settings.s", "notification_channels.#", "1"),
					resource.TestCheckResourceAttr("portainer_alerting_settings.s", "notification_channels.0.channel_id", "0"),
					resource.TestCheckResourceAttr("portainer_alerting_settings.s", "notification_channels.0.config.webhook_url", "https://hooks.slack.com/x"),
					func(*terraform.State) error {
						settings, _ := lastAlertingBody(t, mock, "PUT", "/observability/alerting/settings")["AlertingSettings"].(map[string]interface{})
						channels, _ := settings["notificationChannels"].([]interface{})
						if len(channels) != 1 {
							return fmt.Errorf("expected 1 notificationChannel in the payload, got %v", settings["notificationChannels"])
						}
						ch := channels[0].(map[string]interface{})
						if ch["name"] != "ops-slack" || ch["type"] != "slack" || ch["enabled"] != true {
							return fmt.Errorf("unexpected channel %v", ch)
						}
						return nil
					},
				),
			},
			{
				// An import reads every channel back, as SDKv2 did.
				ResourceName:      "portainer_alerting_settings.s",
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				// Portainer assigns the channel an ID: the refresh reads it.
				PreConfig: func() {
					fake.mu.Lock()
					chans := fake.entries[0]["notificationChannels"].([]interface{})
					chans[0].(map[string]interface{})["id"] = float64(11)
					fake.mu.Unlock()
				},
				RefreshState: true,
				Check:        resource.TestCheckResourceAttr("portainer_alerting_settings.s", "notification_channels.0.channel_id", "11"),
			},
		},
	})
}

// TestAlertingSettings_UpgradeFromV300: settings written by v3.0.0 (SDKv2)
// plan no change with this build, with and without channels.
func TestAlertingSettings_UpgradeFromV300(t *testing.T) {
	t.Parallel()
	for name, body := range map[string]string{
		"minimal": `  enabled = true`,
		"full": `
  enabled       = false
  name          = "am"
  url           = "http://am.example:9093"
  portainer_url = "http://portainer.example"
  notification_channels {
    name    = "ops"
    type    = "webhook"
    enabled = true
    config  = { url = "https://hooks.example/x" }
  }`,
	} {
		t.Run(name, func(t *testing.T) {
			mock := NewMockServer(t)
			fake := &fakeAlertingSettings{}
			fake.register(mock)

			config := mockProviderConfig(mock) + "resource \"portainer_alerting_settings\" \"s\" {\n" + body + "\n}"
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

// TestAccAlertingSettings_Basic enables the internal AlertManager on a
// Business Edition instance.
func TestAccAlertingSettings_Basic(t *testing.T) {
	requireAlertingBE(t)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { accPreCheck(t) },
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{{
			Config: accProvider() + `resource "portainer_alerting_settings" "s" { enabled = true }`,
			Check:  resource.TestCheckResourceAttr("portainer_alerting_settings.s", "enabled", "true"),
		}},
	})
}
