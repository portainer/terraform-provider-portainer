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

// =========================================================================
// Add-on store (Business Edition). These tests pin the wire format against
// the published BE OpenAPI contract: there is no live BE instance to run
// them against, so what they verify is the shape of every request and how
// each response is folded into state.
// =========================================================================

// fakeAddonAPI is a stateful stand-in for one addon ("portal-template") of
// Portainer's addon API: install/inspect/uninstall, access and config.
type fakeAddonAPI struct {
	mu              sync.Mutex
	installed       bool
	noRecord        bool
	registryID      int
	imageRegistryID int
	publicURL       string
	enabled         bool
	users, teams    map[string]map[string]int
	config          []addonConfigEntry
}

const fakeAddonPath = "/addons/portal-template"

func newFakeAddonAPI(m *MockServer) *fakeAddonAPI {
	f := &fakeAddonAPI{enabled: true, users: map[string]map[string]int{}, teams: map[string]map[string]int{}}
	m.On("GET", fakeAddonPath, func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		RespondJSON(http.StatusOK, f.inspect())(w, r)
	})
	m.On("POST", fakeAddonPath, func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			RegistryID      int `json:"registryId"`
			ImageRegistryID int `json:"imageRegistryId"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		f.mu.Lock()
		defer f.mu.Unlock()
		if !f.installed {
			f.registryID = body.RegistryID
		}
		f.imageRegistryID = body.ImageRegistryID
		f.installed = true
		RespondJSON(http.StatusOK, map[string]interface{}{"id": "portal-template", "lifecycleStatus": "installed"})(w, r)
	})
	m.On("PUT", fakeAddonPath+"/public-url", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			PublicURL string `json:"publicUrl"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		f.mu.Lock()
		defer f.mu.Unlock()
		f.publicURL = body.PublicURL
		RespondJSON(http.StatusOK, map[string]interface{}{"id": "portal-template", "publicUrl": f.publicURL})(w, r)
	})
	m.On("DELETE", fakeAddonPath, func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		f.installed = false
		f.registryID, f.imageRegistryID = 0, 0
		RespondJSON(http.StatusOK, map[string]interface{}{"id": "portal-template"})(w, r)
	})
	m.On("PUT", fakeAddonPath+"/access", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Enabled *bool                     `json:"enabled"`
			Users   map[string]map[string]int `json:"userAccessPolicies"`
			Teams   map[string]map[string]int `json:"teamAccessPolicies"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		f.mu.Lock()
		defer f.mu.Unlock()
		if body.Enabled != nil {
			f.enabled = *body.Enabled
		}
		if body.Users != nil {
			f.users = body.Users
		}
		if body.Teams != nil {
			f.teams = body.Teams
		}
		RespondJSON(http.StatusOK, map[string]interface{}{"id": "portal-template"})(w, r)
	})
	m.On("PUT", fakeAddonPath+"/config", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Entries []addonConfigEntry `json:"entries"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		f.mu.Lock()
		defer f.mu.Unlock()
		f.config = body.Entries
		RespondJSON(http.StatusOK, map[string]interface{}{"entries": f.publicConfig()})(w, r)
	})
	m.On("GET", fakeAddonPath+"/config", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		RespondJSON(http.StatusOK, map[string]interface{}{"entries": f.publicConfig()})(w, r)
	})
	m.On("DELETE", fakeAddonPath+"/config", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		f.config = nil
		RespondJSON(http.StatusOK, map[string]interface{}{"entries": []interface{}{}})(w, r)
	})
	for _, key := range []string{"BASE_DOMAIN", "KEEP", "OLD_KEY", "API_TOKEN", "NEW_KEY"} {
		key := key
		m.On("PATCH", fakeAddonPath+"/config/"+key, func(w http.ResponseWriter, r *http.Request) {
			var body struct {
				Value     string `json:"value"`
				Sensitive bool   `json:"sensitive"`
			}
			_ = json.NewDecoder(r.Body).Decode(&body)
			f.mu.Lock()
			defer f.mu.Unlock()
			f.removeConfig(key)
			f.config = append(f.config, addonConfigEntry{Key: key, Value: body.Value, Sensitive: body.Sensitive})
			RespondJSON(http.StatusOK, map[string]interface{}{"entries": f.publicConfig()})(w, r)
		})
		m.On("DELETE", fakeAddonPath+"/config/"+key, func(w http.ResponseWriter, r *http.Request) {
			f.mu.Lock()
			defer f.mu.Unlock()
			f.removeConfig(key)
			RespondJSON(http.StatusOK, map[string]interface{}{"entries": f.publicConfig()})(w, r)
		})
	}
	return f
}

func (f *fakeAddonAPI) removeConfig(key string) {
	kept := f.config[:0]
	for _, e := range f.config {
		if e.Key != key {
			kept = append(kept, e)
		}
	}
	f.config = kept
}

// publicConfig withholds sensitive values, as Portainer does.
func (f *fakeAddonAPI) publicConfig() []addonConfigEntry {
	out := make([]addonConfigEntry, 0, len(f.config))
	for _, e := range f.config {
		if e.Sensitive {
			e.Value = ""
		}
		out = append(out, e)
	}
	return out
}

func (f *fakeAddonAPI) inspect() map[string]interface{} {
	lifecycle := ""
	if f.installed {
		lifecycle = "installed"
	}
	out := map[string]interface{}{
		"id":               "portal-template",
		"displayName":      "Portal",
		"description":      "A portal",
		"shortDescription": "Portal",
		"icon":             "portal.svg",
		"path":             "/portal",
		"enabled":          f.enabled,
		"healthStatus":     "healthy",
		"lifecycleStatus":  lifecycle,
	}
	if !f.noRecord {
		out["record"] = map[string]interface{}{
			"chartVersion":        "1.2.3",
			"chartPath":           "portainer/charts/portal",
			"helmChartRepository": "oci://ghcr.io/portainer/charts/portal-template",
			"registryId":          f.registryID,
			"imageRegistryId":     f.imageRegistryID,
			"availableVersions":   []string{"1.3.0", "1.2.3"},
			"upgradeAvailable":    true,
			"publicUrl":           f.publicURL,
			"userAccessPolicies":  f.users,
			"teamAccessPolicies":  f.teams,
		}
	}
	return out
}

const addonFullConfig = `
resource "portainer_addon" "a" {
  addon_id          = "portal-template"
  chart             = "portainer/charts/portal"
  version           = "1.2.3"
  registry_id       = 3
  image_registry_id = 4
  values            = jsonencode({ ingress = { enabled = true }, replicas = 2 })
}
`

const addonMinimalConfig = `
resource "portainer_addon" "a" {
  addon_id = "portal-template"
}
`

// TestAddonResource_SendsCatalogPayload pins the install payload: camelCase
// keys, values decoded from JSON into a real object rather than passed as a
// string, and imageRegistryId always present. Import keeps the state.
func TestAddonResource_SendsCatalogPayload(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	newFakeAddonAPI(mock)

	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{
				Config: mockProviderConfig(mock) + addonFullConfig,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("portainer_addon.a", "id", "portal-template"),
					resource.TestCheckResourceAttr("portainer_addon.a", "chart_version", "1.2.3"),
					resource.TestCheckResourceAttr("portainer_addon.a", "upgrade_available", "true"),
					resource.TestCheckResourceAttr("portainer_addon.a", "available_versions.#", "2"),
					resource.TestCheckResourceAttr("portainer_addon.a", "lifecycle_status", "installed"),
					func(*terraform.State) error {
						req := mock.FindRequest("POST", fakeAddonPath)
						if req == nil {
							return fmt.Errorf("no install request was sent")
						}
						var payload map[string]interface{}
						if err := req.DecodeJSON(&payload); err != nil {
							return fmt.Errorf("install payload is not JSON: %w", err)
						}
						for key, want := range map[string]interface{}{
							"chart":           "portainer/charts/portal",
							"version":         "1.2.3",
							"registryId":      float64(3),
							"imageRegistryId": float64(4),
						} {
							if payload[key] != want {
								return fmt.Errorf("%s: expected %v, got %v", key, want, payload[key])
							}
						}
						values, ok := payload["values"].(map[string]interface{})
						if !ok {
							return fmt.Errorf("values must be sent as a JSON object, got %T", payload["values"])
						}
						if values["replicas"] != float64(2) {
							return fmt.Errorf("values.replicas: expected 2, got %v", values["replicas"])
						}
						return nil
					},
				),
			},
			{
				ResourceName:            "portainer_addon.a",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"chart", "version", "values"},
			},
		},
	})
}

// TestAddonResource_AlwaysSendsImageRegistry covers the one payload field
// that has to be sent even at zero: omitting it tells Portainer to keep
// whatever registry the addon is installed with, which is not what an absent
// attribute means in a Terraform configuration.
func TestAddonResource_AlwaysSendsImageRegistry(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	newFakeAddonAPI(mock)

	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{{
			Config: mockProviderConfig(mock) + addonMinimalConfig,
			Check: resource.ComposeTestCheckFunc(
				resource.TestCheckNoResourceAttr("portainer_addon.a", "registry_id"),
				resource.TestCheckNoResourceAttr("portainer_addon.a", "image_registry_id"),
				func(*terraform.State) error {
					var payload map[string]interface{}
					if err := mock.FindRequest("POST", fakeAddonPath).DecodeJSON(&payload); err != nil {
						return fmt.Errorf("install payload is not JSON: %w", err)
					}
					if _, ok := payload["imageRegistryId"]; !ok {
						return fmt.Errorf("imageRegistryId must be sent even when unset, to mean 'no credentials'")
					}
					// The optional fields, in contrast, stay out of the payload entirely.
					for _, key := range []string{"chart", "version", "registryId", "values"} {
						if _, ok := payload[key]; ok {
							return fmt.Errorf("%s must be omitted when not configured, got %v", key, payload[key])
						}
					}
					if mock.FindRequest("PUT", fakeAddonPath+"/public-url") != nil {
						return fmt.Errorf("the public URL must not be set when public_url is not configured")
					}
					return nil
				},
			),
		}},
	})
}

// TestAddonResource_UninstalledOutsideTerraform covers the drift case that a
// 404 does not cover: an addon uninstalled outside Terraform stays in the
// catalog, and only its empty lifecycle status says the release is gone.
func TestAddonResource_UninstalledOutsideTerraform(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	fake := newFakeAddonAPI(mock)

	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{Config: mockProviderConfig(mock) + addonMinimalConfig},
			{
				PreConfig: func() {
					fake.mu.Lock()
					fake.installed = false
					fake.mu.Unlock()
				},
				Config:             mockProviderConfig(mock) + addonMinimalConfig,
				PlanOnly:           true,
				ExpectNonEmptyPlan: true,
			},
		},
	})
}

// TestAddonResource_TolerantOfMissingRecord guards the non-admin case: the
// record is admin-only and simply absent, which must not fail the read nor
// plan a change.
func TestAddonResource_TolerantOfMissingRecord(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	fake := newFakeAddonAPI(mock)
	fake.noRecord = true

	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{
				Config: mockProviderConfig(mock) + addonFullConfig,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("portainer_addon.a", "id", "portal-template"),
					resource.TestCheckResourceAttr("portainer_addon.a", "registry_id", "3"),
					resource.TestCheckResourceAttr("portainer_addon.a", "chart_version", ""),
				),
			},
			{
				Config:   mockProviderConfig(mock) + addonFullConfig,
				PlanOnly: true,
			},
		},
	})
}

// TestAddonResource_UninstallPassesFlagsAsQuery pins the uninstall flags,
// which Portainer takes as query parameters rather than a body.
func TestAddonResource_UninstallPassesFlagsAsQuery(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	newFakeAddonAPI(mock)

	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		CheckDestroy: func(*terraform.State) error {
			req := mock.FindRequest("DELETE", fakeAddonPath)
			if req == nil {
				return fmt.Errorf("no uninstall request was sent")
			}
			if !strings.Contains(req.Query, "force=true") || !strings.Contains(req.Query, "pruneConfig=true") {
				return fmt.Errorf("expected both uninstall flags in the query, got %q", req.Query)
			}
			return nil
		},
		Steps: []resource.TestStep{{
			Config: mockProviderConfig(mock) + `
resource "portainer_addon" "a" {
  addon_id                  = "portal-template"
  force_uninstall           = true
  prune_config_on_uninstall = true
}
`,
		}},
	})
}

// TestAddonResource_ValuesRejectsNonObject keeps a JSON array or scalar from
// reaching the server, where it would come back as an opaque decode failure.
func TestAddonResource_ValuesRejectsNonObject(t *testing.T) {
	t.Parallel()
	if _, errs := validateJSONObject(`{"a":1}`, "values"); len(errs) != 0 {
		t.Errorf("a JSON object must validate, got %v", errs)
	}
	if _, errs := validateJSONObject(`not json`, "values"); len(errs) == 0 {
		t.Error("invalid JSON must be rejected")
	}

	mock := NewMockServer(t)
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{{
			Config: mockProviderConfig(mock) + `
resource "portainer_addon" "a" {
  addon_id = "portal-template"
  values   = jsonencode([1, 2])
}
`,
			ExpectError: regexp.MustCompile(`must be a JSON object`),
		}},
	})
}

// TestAddonResource_UpgradeFromV300: state written by v3.0.0 (SDKv2) against
// the same fake plans no change with this build, with every optional
// argument set and with none.
func TestAddonResource_UpgradeFromV300(t *testing.T) {
	t.Parallel()
	for name, cfg := range map[string]string{"full": addonFullConfig, "minimal": addonMinimalConfig} {
		t.Run(name, func(t *testing.T) {
			mock := NewMockServer(t)
			newFakeAddonAPI(mock)
			config := mockProviderConfig(mock) + cfg
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

// TestAccAddonsDataSource needs a Business Edition with addon support, which
// means Portainer itself running on Kubernetes (GET /system/info reports
// addonsAvailable = "available"); installing an addon (portainer_addon)
// additionally needs a Kubernetes environment.
func TestAccAddonsDataSource(t *testing.T) {
	if os.Getenv("PORTAINER_ACC_BE") != "1" {
		t.Skip("portainer_addon needs Portainer Business Edition with a Kubernetes environment for addons; set PORTAINER_ACC_BE=1")
	}
	accPreCheck(t)
	if got := accSystemInfo(t)["addonsAvailable"]; got != addonsAvailable && got != true {
		t.Skipf("add-ons are not available on this Portainer (GET /system/info addonsAvailable = %v); they need Portainer running on Kubernetes", got)
	}
	resource.Test(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{{
			Config: accProvider() + `data "portainer_addons" "all" {}`,
			Check:  resource.TestCheckResourceAttrSet("data.portainer_addons.all", "id"),
		}},
	})
}

// TestAddonResource_PublicURL covers public_url (Business Edition 3.1): it is
// set with PUT /addons/{id}/public-url without reinstalling the addon,
// detected as drift while configured, and cleared when removed from the
// configuration. An addon without it never calls the endpoint (see
// TestAddonResource_AlwaysSendsImageRegistry's minimal configuration).
func TestAddonResource_PublicURL(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	fake := newFakeAddonAPI(mock)
	withURL := func(u string) string {
		return mockProviderConfig(mock) + fmt.Sprintf(`
resource "portainer_addon" "a" {
  addon_id   = "portal-template"
  public_url = %q
}
`, u)
	}
	countRequests := func(method, path string) int {
		n := 0
		for _, r := range mock.Requests() {
			if r.Method == method && r.Path == path {
				n++
			}
		}
		return n
	}
	lastPublicURL := func() (string, error) {
		var body map[string]interface{}
		var last *RecordedRequest
		for _, r := range mock.Requests() {
			if r.Method == http.MethodPut && r.Path == fakeAddonPath+"/public-url" {
				last = r
			}
		}
		if last == nil {
			return "", fmt.Errorf("no public URL request was sent")
		}
		if err := last.DecodeJSON(&body); err != nil {
			return "", err
		}
		v, ok := body["publicUrl"].(string)
		if !ok {
			return "", fmt.Errorf("publicUrl missing from %v", body)
		}
		return v, nil
	}

	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{
				Config: withURL("https://run.example.com"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("portainer_addon.a", "public_url", "https://run.example.com"),
					func(*terraform.State) error {
						if got, err := lastPublicURL(); err != nil || got != "https://run.example.com" {
							return fmt.Errorf("publicUrl: got %q (%w)", got, err)
						}
						return nil
					},
				),
			},
			{
				// Changing only the URL must not reinstall the addon.
				Config: withURL("https://portal.example.com"),
				Check: func(*terraform.State) error {
					if n := countRequests("POST", fakeAddonPath); n != 1 {
						return fmt.Errorf("expected the addon to be installed once, got %d install requests", n)
					}
					if got, err := lastPublicURL(); err != nil || got != "https://portal.example.com" {
						return fmt.Errorf("publicUrl: got %q (%w)", got, err)
					}
					return nil
				},
			},
			{
				PreConfig: func() {
					fake.mu.Lock()
					fake.publicURL = "https://elsewhere.example.com"
					fake.mu.Unlock()
				},
				Config:             withURL("https://portal.example.com"),
				PlanOnly:           true,
				ExpectNonEmptyPlan: true,
			},
			{
				Config: mockProviderConfig(mock) + addonMinimalConfig,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckNoResourceAttr("portainer_addon.a", "public_url"),
					func(*terraform.State) error {
						if got, err := lastPublicURL(); err != nil || got != "" {
							return fmt.Errorf("removing public_url must clear it, got %q (%w)", got, err)
						}
						return nil
					},
				),
			},
		},
	})
}
