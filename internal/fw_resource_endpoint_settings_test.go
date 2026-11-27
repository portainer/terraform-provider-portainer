package internal

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"strings"
	"sync"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// lastRequestA2 returns the last recorded request matching method+path.
func lastRequestA2(m *MockServer, method, path string) *RecordedRequest {
	var found *RecordedRequest
	for _, r := range m.Requests() {
		if r.Method == method && r.Path == path {
			found = r
		}
	}
	return found
}

// countRequestsA2 counts the recorded requests matching method+path.
func countRequestsA2(m *MockServer, method, path string) int {
	n := 0
	for _, r := range m.Requests() {
		if r.Method == method && r.Path == path {
			n++
		}
	}
	return n
}

// fakeEndpointSettings is a stateful stand-in for GET /endpoints/{id} and
// PUT /endpoints/{id}/settings of one environment.
type fakeEndpointSettings struct {
	mu    sync.Mutex
	id    int
	gone  bool
	state endpointSettingsState
}

func (f *fakeEndpointSettings) register(m *MockServer) {
	path := fmt.Sprintf("/endpoints/%d", f.id)
	m.On("GET", path, func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		if f.gone {
			RespondJSON(http.StatusNotFound, map[string]string{"message": "not found"})(w, r)
			return
		}
		RespondJSON(http.StatusOK, map[string]interface{}{
			"Id":                  f.id,
			"EnableGPUManagement": f.state.EnableGPUManagement,
			"Gpus":                f.state.Gpus,
			"SecuritySettings":    f.state.SecuritySettings,
		})(w, r)
	})
	m.On("PUT", path+"/settings", func(w http.ResponseWriter, r *http.Request) {
		var p EndpointSettingsPayload
		_ = json.NewDecoder(r.Body).Decode(&p)
		f.mu.Lock()
		defer f.mu.Unlock()
		f.state.EnableGPUManagement = p.EnableGPUManagement
		if p.GPUs != nil {
			f.state.Gpus = p.GPUs
		}
		if p.AllowBindMountsForRegularUsers != nil {
			f.state.SecuritySettings = &SecuritySettings{
				AllowBindMountsForRegularUsers:            *p.AllowBindMountsForRegularUsers,
				AllowContainerCapabilitiesForRegularUsers: *p.AllowContainerCapabilitiesForRegularUsers,
				AllowDeviceMappingForRegularUsers:         *p.AllowDeviceMappingForRegularUsers,
				AllowHostNamespaceForRegularUsers:         *p.AllowHostNamespaceForRegularUsers,
				AllowPrivilegedModeForRegularUsers:        *p.AllowPrivilegedModeForRegularUsers,
				AllowStackManagementForRegularUsers:       *p.AllowStackManagementForRegularUsers,
				AllowSysctlSettingForRegularUsers:         *p.AllowSysctlSettingForRegularUsers,
				AllowSecurityOptForRegularUsers:           *p.AllowSecurityOptForRegularUsers,
				AllowVolumeBrowserForRegularUsers:         *p.AllowVolumeBrowserForRegularUsers,
				EnableHostManagementFeatures:              *p.EnableHostManagementFeatures,
			}
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("{}"))
	})
}

func newFakeEndpointSettings(m *MockServer, id int) *fakeEndpointSettings {
	f := &fakeEndpointSettings{id: id, state: endpointSettingsState{SecuritySettings: &SecuritySettings{}}}
	f.register(m)
	return f
}

const endpointSettingsFullConfig = `
resource "portainer_endpoint_settings" "s" {
  endpoint_id               = 7
  enable_gpu_management     = true
  enable_image_notification = true
  gpus {
    name  = "gpu0"
    value = "nvidia-0"
  }
  change_window {
    enabled    = true
    start_time = "01:00"
    end_time   = "03:00"
  }
  deployment_options {
    hide_add_with_form      = true
    hide_web_editor         = true
    override_global_options = true
  }
  security_settings {
    allow_bind_mounts      = true
    allow_device_mapping   = true
    allow_volume_browser   = true
    enable_host_management = true
  }
}
`

const endpointSettingsMinimalConfig = `resource "portainer_endpoint_settings" "s" { endpoint_id = 7 }`

func decodeA2(t *testing.T, r *RecordedRequest) map[string]interface{} {
	t.Helper()
	if r == nil {
		t.Fatal("expected request was not sent")
	}
	var payload map[string]interface{}
	if err := r.DecodeJSON(&payload); err != nil {
		t.Fatalf("failed to decode body: %v", err)
	}
	return payload
}

// TestEndpointSettingsResource_Lifecycle: the full payload shape (camelCase
// top-level keys, nested objects and the top-level security switches CE
// reads, including the ones left false), an empty plan, and import.
func TestEndpointSettingsResource_Lifecycle(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	newFakeEndpointSettings(mock, 7)

	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{
				Config: mockProviderConfig(mock) + endpointSettingsFullConfig,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("portainer_endpoint_settings.s", "id", "7"),
					resource.TestCheckResourceAttr("portainer_endpoint_settings.s", "security_settings.0.allow_privileged_mode", "false"),
					resource.TestCheckResourceAttr("portainer_endpoint_settings.s", "change_window.0.start_time", "01:00"),
					func(_ *terraform.State) error {
						payload := decodeA2(t, mock.FindRequest("PUT", "/endpoints/7/settings"))
						if payload["enableGPUManagement"] != true || payload["enableImageNotification"] != true {
							return fmt.Errorf("flags not sent: %v", payload)
						}
						gpus, _ := payload["gpus"].([]interface{})
						if len(gpus) != 1 || gpus[0].(map[string]interface{})["name"] != "gpu0" {
							return fmt.Errorf("gpus: %v", payload["gpus"])
						}
						cw, _ := payload["changeWindow"].(map[string]interface{})
						if cw["Enabled"] != true || cw["StartTime"] != "01:00" || cw["EndTime"] != "03:00" {
							return fmt.Errorf("changeWindow: %v", cw)
						}
						dep, _ := payload["deploymentOptions"].(map[string]interface{})
						if dep["hideAddWithForm"] != true || dep["hideWebEditor"] != true {
							return fmt.Errorf("deploymentOptions: %v", dep)
						}
						sec, _ := payload["securitySettings"].(map[string]interface{})
						if sec["allowBindMountsForRegularUsers"] != true || sec["enableHostManagementFeatures"] != true {
							return fmt.Errorf("securitySettings: %v", sec)
						}
						want := map[string]bool{
							"allowBindMountsForRegularUsers":            true,
							"allowContainerCapabilitiesForRegularUsers": false,
							"allowDeviceMappingForRegularUsers":         true,
							"allowHostNamespaceForRegularUsers":         false,
							"allowPrivilegedModeForRegularUsers":        false,
							"allowStackManagementForRegularUsers":       false,
							"allowSysctlSettingForRegularUsers":         false,
							"allowSecurityOptForRegularUsers":           false,
							"allowVolumeBrowserForRegularUsers":         true,
							"enableHostManagementFeatures":              true,
						}
						for key, value := range want {
							if got, ok := payload[key]; !ok || got != value {
								return fmt.Errorf("%s: expected %v at the top level, got %v (present=%v)", key, value, got, ok)
							}
						}
						return nil
					},
				),
			},
			{
				ResourceName:      "portainer_endpoint_settings.s",
				ImportState:       true,
				ImportStateVerify: true,
				// Business Edition fields Portainer CE does not return.
				ImportStateVerifyIgnore: []string{"change_window", "deployment_options", "enable_image_notification"},
			},
		},
	})
}

// TestEndpointSettingsResource_Minimal: no blocks sends the defaults, no
// security switch and a null GPU list, so Portainer keeps what it has.
func TestEndpointSettingsResource_Minimal(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	newFakeEndpointSettings(mock, 7)

	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{{
			Config: mockProviderConfig(mock) + endpointSettingsMinimalConfig,
			Check: func(_ *terraform.State) error {
				payload := decodeA2(t, mock.FindRequest("PUT", "/endpoints/7/settings"))
				if payload["enableGPUManagement"] != false {
					return fmt.Errorf("enableGPUManagement default: got %v", payload["enableGPUManagement"])
				}
				if gpus, present := payload["gpus"]; !present || gpus != nil {
					return fmt.Errorf("gpus: expected null so Portainer keeps them, got %v (present=%v)", gpus, present)
				}
				if _, ok := payload["allowStackManagementForRegularUsers"]; ok {
					return fmt.Errorf("expected no security switches without a security_settings block, got %v", payload)
				}
				return nil
			},
		}},
	})
}

// TestEndpointSettingsResource_GPUList: GPUs removed from the configuration
// are cleared in Portainer (an empty list); unchanged managed GPUs are sent.
func TestEndpointSettingsResource_GPUList(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	newFakeEndpointSettings(mock, 7)
	withGPU := `resource "portainer_endpoint_settings" "s" {
  endpoint_id = 7
  gpus {
    name  = "nvidia"
    value = "gpu0"
  }
}`

	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{Config: mockProviderConfig(mock) + withGPU},
			{
				Config: mockProviderConfig(mock) + strings.Replace(withGPU, "endpoint_id = 7", "endpoint_id = 7\n  enable_gpu_management = true", 1),
				Check: func(_ *terraform.State) error {
					gpus, _ := decodeA2(t, lastRequestA2(mock, "PUT", "/endpoints/7/settings"))["gpus"].([]interface{})
					if len(gpus) != 1 {
						return fmt.Errorf("gpus: expected the managed list, got %v", gpus)
					}
					return nil
				},
			},
			{
				Config: mockProviderConfig(mock) + `resource "portainer_endpoint_settings" "s" {
  endpoint_id           = 7
  enable_gpu_management = true
}`,
				Check: func(_ *terraform.State) error {
					gpus, ok := decodeA2(t, lastRequestA2(mock, "PUT", "/endpoints/7/settings"))["gpus"].([]interface{})
					if !ok || len(gpus) != 0 {
						return fmt.Errorf("gpus: expected an empty list to clear them, got %v", gpus)
					}
					return nil
				},
			},
		},
	})
}

// TestEndpointSettingsResource_Drift: a security switch flipped in the
// Portainer UI shows up in the plan; GPUs the configuration does not manage
// do not.
func TestEndpointSettingsResource_Drift(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	fake := newFakeEndpointSettings(mock, 7)
	config := mockProviderConfig(mock) + `resource "portainer_endpoint_settings" "s" {
  endpoint_id = 7
  security_settings {
    allow_bind_mounts = false
  }
}`

	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{Config: config},
			{
				PreConfig: func() {
					fake.mu.Lock()
					fake.state.Gpus = []GPU{{Name: "gpu0", Value: "0"}}
					fake.mu.Unlock()
				},
				Config: config,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
			{
				PreConfig: func() {
					fake.mu.Lock()
					fake.state.SecuritySettings.AllowBindMountsForRegularUsers = true
					fake.mu.Unlock()
				},
				Config:             config,
				PlanOnly:           true,
				ExpectNonEmptyPlan: true,
			},
		},
	})
}

// TestEndpointSettingsResource_EnvironmentGone: the resource leaves state with
// its environment.
func TestEndpointSettingsResource_EnvironmentGone(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	fake := newFakeEndpointSettings(mock, 7)
	config := mockProviderConfig(mock) + endpointSettingsMinimalConfig

	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{Config: config},
			{
				PreConfig: func() {
					fake.mu.Lock()
					fake.gone = true
					fake.mu.Unlock()
				},
				Config:             config,
				PlanOnly:           true,
				ExpectNonEmptyPlan: true,
			},
		},
	})
}

func TestEndpointSettingsResource_HTTPError(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("PUT", "/endpoints/9/settings", RespondString(http.StatusBadRequest, "application/json", `{"message":"bad request"}`))

	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{{
			Config:      mockProviderConfig(mock) + `resource "portainer_endpoint_settings" "s" { endpoint_id = 9 }`,
			ExpectError: regexp.MustCompile(`failed to update endpoint settings`),
		}},
	})
}

func TestEndpointSettingsResource_ImportRejectsBadID(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{{
			Config:        mockProviderConfig(mock) + endpointSettingsMinimalConfig,
			ResourceName:  "portainer_endpoint_settings.s",
			ImportState:   true,
			ImportStateId: "abc",
			ExpectError:   regexp.MustCompile(`expected the numeric environment ID`),
		}},
	})
}

// TestEndpointSettingsResource_UpgradeFromV300: state written by v3.0.0 plans
// no change with this build, with every block set and with none.
func TestEndpointSettingsResource_UpgradeFromV300(t *testing.T) {
	t.Parallel()
	for name, body := range map[string]string{
		"full":    endpointSettingsFullConfig,
		"minimal": endpointSettingsMinimalConfig,
		"partial blocks": `resource "portainer_endpoint_settings" "s" {
  endpoint_id = 7
  change_window {
    enabled = true
  }
  security_settings {
    allow_bind_mounts = true
  }
}`,
	} {
		t.Run(name, func(t *testing.T) {
			mock := NewMockServer(t)
			newFakeEndpointSettings(mock, 7)
			config := mockProviderConfig(mock) + body
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

// TestAccEndpointSettingsResource needs a Docker environment to change.
func TestAccEndpointSettingsResource(t *testing.T) {
	envID := os.Getenv("PORTAINER_ACC_SWARM_ENV_ID")
	if envID == "" {
		t.Skip("PORTAINER_ACC_SWARM_ENV_ID must name a Docker environment whose settings may be changed")
	}
	config := fmt.Sprintf(`resource "portainer_endpoint_settings" "s" {
  endpoint_id = %s
  security_settings {
    allow_bind_mounts = true
  }
}`, envID)
	resource.Test(t, resource.TestCase{
		PreCheck: func() { accPreCheck(t) },
		Steps: []resource.TestStep{
			{ProtoV5ProviderFactories: testProtoV5Factories, Config: accProvider() + config},
		},
	})
}
