package internal

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"sync"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// Business Edition settings endpoints. Each of these lives outside
// PUT /settings, which is why they are separate resources: folding them in
// would make every settings apply rewrite them too.

// fakeToggle stores the bool a PUT sent under putKey and serves it from a GET
// built by wrap.
type fakeToggle struct {
	mu    sync.Mutex
	value bool
}

func (f *fakeToggle) register(m *MockServer, putPath, putKey, getPath string, wrap func(bool) interface{}) {
	m.On("PUT", putPath, func(w http.ResponseWriter, r *http.Request) {
		var body map[string]interface{}
		_ = json.NewDecoder(r.Body).Decode(&body)
		f.mu.Lock()
		f.value, _ = body[putKey].(bool)
		f.mu.Unlock()
		RespondJSON(http.StatusOK, map[string]interface{}{})(w, r)
	})
	m.On("GET", getPath, func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		RespondJSON(http.StatusOK, wrap(f.value))(w, r)
	})
}

func (f *fakeToggle) set(v bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.value = v
}

func defaultRegistryMock(t *testing.T) (*MockServer, *fakeToggle) {
	mock := NewMockServer(t)
	fake := &fakeToggle{}
	fake.register(mock, "/settings/default_registry", "Hide", "/settings", func(v bool) interface{} {
		return map[string]interface{}{"DefaultRegistry": map[string]interface{}{"Hide": v}}
	})
	return mock, fake
}

func additionalFunctionalityMock(t *testing.T) (*MockServer, *fakeToggle) {
	mock := NewMockServer(t)
	fake := &fakeToggle{}
	fake.register(mock, "/settings/additional_functionality", "Policies", "/settings/additional_functionality", func(v bool) interface{} {
		return map[string]interface{}{"additionalFunctionality": map[string]interface{}{"Policies": v}}
	})
	return mock, fake
}

func experimentalMock(t *testing.T) (*MockServer, *fakeToggle) {
	mock := NewMockServer(t)
	fake := &fakeToggle{}
	fake.register(mock, "/settings/experimental", "openAIIntegration", "/settings/experimental", func(v bool) interface{} {
		return map[string]interface{}{"experimentalFeatures": map[string]interface{}{"OpenAIIntegration": v}}
	})
	return mock, fake
}

// TestSettingsDefaultRegistryResource_Lifecycle pins the payload key, which is
// PascalCase where most of the provider's payloads are camelCase, that the
// current value is read from the main settings object (the endpoint itself is
// write-only), drift, import, and that destroy leaves the setting alone.
func TestSettingsDefaultRegistryResource_Lifecycle(t *testing.T) {
	t.Parallel()
	mock, fake := defaultRegistryMock(t)
	config := mockProviderConfig(mock) + `resource "portainer_settings_default_registry" "r" { hide = true }`

	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{
				Config: config,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("portainer_settings_default_registry.r", "id", "portainer-settings-default-registry"),
					resource.TestCheckResourceAttr("portainer_settings_default_registry.r", "hide", "true"),
					func(*terraform.State) error {
						if p := gsbBody(t, mock, "PUT", "/settings/default_registry"); p["Hide"] != true {
							return fmt.Errorf("expected the PascalCase key Hide set to true, got %v", p)
						}
						return nil
					},
				),
			},
			{PreConfig: func() { fake.set(false) }, Config: config, PlanOnly: true, ExpectNonEmptyPlan: true},
			{Config: config},
			{ResourceName: "portainer_settings_default_registry.r", ImportState: true, ImportStateId: "portainer-settings-default-registry", ImportStateVerify: true},
		},
		CheckDestroy: func(*terraform.State) error {
			if n := gsbCount(mock, "PUT", "/settings/default_registry"); n != 2 {
				return fmt.Errorf("destroy must not call Portainer, got %d updates", n)
			}
			return nil
		},
	})
}

// TestSettingsAdditionalFunctionalityResource_RoundTrips covers the policy
// engine toggle, whose response nests the value one level deeper than the
// payload.
func TestSettingsAdditionalFunctionalityResource_RoundTrips(t *testing.T) {
	t.Parallel()
	mock, _ := additionalFunctionalityMock(t)
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{
				Config: mockProviderConfig(mock) + `resource "portainer_settings_additional_functionality" "a" { policies = true }`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("portainer_settings_additional_functionality.a", "policies", "true"),
					resource.TestCheckResourceAttr("portainer_settings_additional_functionality.a", "id", "portainer-settings-additional-functionality"),
					func(*terraform.State) error {
						if p := gsbBody(t, mock, "PUT", "/settings/additional_functionality"); p["Policies"] != true {
							return fmt.Errorf("expected the required key Policies set to true, got %v", p)
						}
						return nil
					},
				),
			},
			{
				Config: mockProviderConfig(mock) + `resource "portainer_settings_additional_functionality" "a" { policies = false }`,
				Check:  resource.TestCheckResourceAttr("portainer_settings_additional_functionality.a", "policies", "false"),
			},
		},
	})
}

// TestExperimentalSettingsResource_Lifecycle: create, update, the false
// default, import and errors of PUT /settings/experimental.
func TestExperimentalSettingsResource_Lifecycle(t *testing.T) {
	t.Parallel()
	mock, fake := experimentalMock(t)
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{
				Config: mockProviderConfig(mock) + `resource "portainer_settings_experimental" "e" { openai_integration = true }`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("portainer_settings_experimental.e", "id", "portainer-experimental-settings"),
					func(*terraform.State) error {
						if p := gsbBody(t, mock, "PUT", "/settings/experimental"); p["openAIIntegration"] != true {
							return fmt.Errorf("openAIIntegration: got %v", p["openAIIntegration"])
						}
						return nil
					},
				),
			},
			{
				// Unset falls back to false, which is sent.
				Config: mockProviderConfig(mock) + `resource "portainer_settings_experimental" "e" {}`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("portainer_settings_experimental.e", "openai_integration", "false"),
					func(*terraform.State) error {
						if p := gsbBody(t, mock, "PUT", "/settings/experimental"); p["openAIIntegration"] != false {
							return fmt.Errorf("openAIIntegration: got %v", p["openAIIntegration"])
						}
						return nil
					},
				),
			},
			{ResourceName: "portainer_settings_experimental.e", ImportState: true, ImportStateId: "portainer-experimental-settings", ImportStateVerify: true},
			{
				// openai_integration is not read back (Portainer 2.39 and later
				// have no OpenAI integration and do not report it), so a change
				// on the server plans nothing.
				PreConfig: func() { fake.set(true) },
				Config:    mockProviderConfig(mock) + `resource "portainer_settings_experimental" "e" {}`,
				PlanOnly:  true,
			},
		},
	})
}

func TestSettingsToggleResources_HTTPError(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct{ path, config, want string }{
		"default_registry":         {"/settings/default_registry", `resource "portainer_settings_default_registry" "r" { hide = true }`, "failed to update the default registry settings"},
		"additional_functionality": {"/settings/additional_functionality", `resource "portainer_settings_additional_functionality" "r" { policies = true }`, "failed to update the additional functionality settings"},
		"experimental":             {"/settings/experimental", `resource "portainer_settings_experimental" "r" { openai_integration = true }`, "failed to apply experimental settings"},
	} {
		t.Run(name, func(t *testing.T) {
			mock := NewMockServer(t)
			mock.On("PUT", tc.path, RespondString(http.StatusInternalServerError, "application/json", `{"message":"boom"}`))
			resource.UnitTest(t, resource.TestCase{
				ProtoV5ProviderFactories: testProtoV5Factories,
				Steps: []resource.TestStep{{
					Config:      mockProviderConfig(mock) + tc.config,
					ExpectError: regexp.MustCompile(tc.want),
				}},
			})
		})
	}
}

func TestSettingsDefaultRegistryResource_UpgradeFromV300(t *testing.T) {
	t.Parallel()
	newMock := func(t *testing.T) *MockServer { m, _ := defaultRegistryMock(t); return m }
	gsbUpgradeTest(t, newMock,
		`resource "portainer_settings_default_registry" "r" { hide = true }`,
		`resource "portainer_settings_default_registry" "r" { hide = false }`,
	)
}

func TestSettingsAdditionalFunctionalityResource_UpgradeFromV300(t *testing.T) {
	t.Parallel()
	newMock := func(t *testing.T) *MockServer { m, _ := additionalFunctionalityMock(t); return m }
	gsbUpgradeTest(t, newMock,
		`resource "portainer_settings_additional_functionality" "a" { policies = true }`,
	)
}

func TestExperimentalSettingsResource_UpgradeFromV300(t *testing.T) {
	t.Parallel()
	newMock := func(t *testing.T) *MockServer { m, _ := experimentalMock(t); return m }
	gsbUpgradeTest(t, newMock,
		`resource "portainer_settings_experimental" "e" { openai_integration = true }`,
		`resource "portainer_settings_experimental" "e" {}`,
	)
}

// TestAccSettingsExperimentalResource runs against a real (BE) Portainer.
func TestAccSettingsExperimentalResource(t *testing.T) {
	accRequireBE(t, "experimental settings")
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { accPreCheck(t) },
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{
				Config: accProvider() + `resource "portainer_settings_experimental" "e" { openai_integration = false }`,
				Check:  resource.TestCheckResourceAttr("portainer_settings_experimental.e", "openai_integration", "false"),
			},
			{ResourceName: "portainer_settings_experimental.e", ImportState: true, ImportStateId: "portainer-experimental-settings", ImportStateVerify: true},
		},
	})
}

// fleetExperimentalMock is GET/PUT /settings/experimental: a PUT takes both
// switches (one without fleetManagement leaves it as it is) and a GET does not
// report fleet management, as Business Edition 2.39-3.1 answer (live servers
// return an empty experimentalFeatures object; the provider reads neither
// switch back).
func fleetExperimentalMock(t *testing.T, fleet bool) (*MockServer, *fakeToggle) {
	mock := NewMockServer(t)
	f := &fakeToggle{value: fleet}
	var openAI bool
	mock.On("PUT", "/settings/experimental", func(w http.ResponseWriter, r *http.Request) {
		var body map[string]interface{}
		_ = json.NewDecoder(r.Body).Decode(&body)
		f.mu.Lock()
		openAI, _ = body["openAIIntegration"].(bool)
		if v, ok := body["fleetManagement"].(bool); ok {
			f.value = v
		}
		f.mu.Unlock()
		RespondJSON(http.StatusOK, map[string]interface{}{})(w, r)
	})
	mock.On("GET", "/settings/experimental", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		RespondJSON(http.StatusOK, map[string]interface{}{"experimentalFeatures": map[string]interface{}{"OpenAIIntegration": openAI}})(w, r)
	})
	return mock, f
}

// TestExperimentalSettingsResource_FleetManagement: fleet_management is sent
// only when configured, kept in state as configured (Portainer does not report
// it back, so the plan stays empty) and left alone once removed from the
// configuration.
func TestExperimentalSettingsResource_FleetManagement(t *testing.T) {
	t.Parallel()
	mock, fake := fleetExperimentalMock(t, false)
	fleetSent := func(want interface{}) resource.TestCheckFunc {
		return func(*terraform.State) error {
			p := gsbBody(t, mock, "PUT", "/settings/experimental")
			if got, ok := p["fleetManagement"]; (want == nil && ok) || (want != nil && got != want) {
				return fmt.Errorf("fleetManagement: want %v, got %v (present=%v)", want, got, ok)
			}
			return nil
		}
	}
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{
				Config: mockProviderConfig(mock) + `resource "portainer_settings_experimental" "e" { fleet_management = true }`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("portainer_settings_experimental.e", "fleet_management", "true"),
					fleetSent(true),
				),
			},
			{
				PreConfig: func() { fake.set(false) },
				Config:    mockProviderConfig(mock) + `resource "portainer_settings_experimental" "e" { fleet_management = true }`,
				PlanOnly:  true,
			},
			{
				Config: mockProviderConfig(mock) + `resource "portainer_settings_experimental" "e" { openai_integration = true }`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckNoResourceAttr("portainer_settings_experimental.e", "fleet_management"),
					fleetSent(nil),
				),
			},
		},
	})
}

// TestExperimentalSettingsResource_FleetManagement_UpgradeFromV300: a server
// with fleet management on plans no change after upgrading.
func TestExperimentalSettingsResource_FleetManagement_UpgradeFromV300(t *testing.T) {
	t.Parallel()
	newMock := func(t *testing.T) *MockServer { m, _ := fleetExperimentalMock(t, true); return m }
	gsbUpgradeTest(t, newMock,
		`resource "portainer_settings_experimental" "e" { openai_integration = true }`,
		`resource "portainer_settings_experimental" "e" {}`,
	)
}
