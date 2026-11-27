package internal

import (
	"fmt"
	"os"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// =========================================================================
// Add-on access and configuration (Business Edition).
// =========================================================================

const addonAccessFullConfig = `
resource "portainer_addon_access" "a" {
  addon_id = "portal-template"
  enabled  = true
  user_access {
    user_id = 3
    role_id = 2
  }
  team_access {
    team_id = 5
    role_id = 1
  }
}
`

const addonAccessMinimalConfig = `
resource "portainer_addon_access" "a" {
  addon_id = "portal-template"
}
`

// TestAddonAccessResource_SendsPolicyObjects pins the access payload:
// Portainer stores the grants as {"<id>": {"RoleId": n}} objects keyed by
// identifier, not as the lists a Terraform block naturally suggests. Destroy
// revokes the grants but leaves `enabled` alone: giving up managing who may
// use an addon is not the same decision as switching the addon off.
func TestAddonAccessResource_SendsPolicyObjects(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	newFakeAddonAPI(mock)

	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		CheckDestroy: func(*terraform.State) error {
			reqs := addonRequests(mock, "PUT", fakeAddonPath+"/access")
			var payload map[string]interface{}
			if err := reqs[len(reqs)-1].DecodeJSON(&payload); err != nil {
				return fmt.Errorf("revoke payload is not JSON: %w", err)
			}
			if _, ok := payload["enabled"]; ok {
				return fmt.Errorf("destroying the access resource must not switch the addon off")
			}
			for _, key := range []string{"userAccessPolicies", "teamAccessPolicies"} {
				if p, ok := payload[key].(map[string]interface{}); !ok || len(p) != 0 {
					return fmt.Errorf("%s: expected an empty object, got %v", key, payload[key])
				}
			}
			return nil
		},
		Steps: []resource.TestStep{
			{
				Config: mockProviderConfig(mock) + addonAccessFullConfig,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("portainer_addon_access.a", "id", "portal-template"),
					resource.TestCheckResourceAttr("portainer_addon_access.a", "user_access.#", "1"),
					func(*terraform.State) error {
						var payload struct {
							Enabled bool                      `json:"enabled"`
							Users   map[string]map[string]int `json:"userAccessPolicies"`
							Teams   map[string]map[string]int `json:"teamAccessPolicies"`
						}
						if err := mock.FindRequest("PUT", fakeAddonPath+"/access").DecodeJSON(&payload); err != nil {
							return fmt.Errorf("access payload is not JSON: %w", err)
						}
						if !payload.Enabled {
							return fmt.Errorf("enabled must be sent")
						}
						if got := payload.Users["3"]["RoleId"]; got != 2 {
							return fmt.Errorf("user 3: expected RoleId 2, got %d", got)
						}
						if got := payload.Teams["5"]["RoleId"]; got != 1 {
							return fmt.Errorf("team 5: expected RoleId 1, got %d", got)
						}
						return nil
					},
				),
			},
			{
				ResourceName:      "portainer_addon_access.a",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func addonRequests(m *MockServer, method, path string) []*RecordedRequest {
	var out []*RecordedRequest
	for _, r := range m.Requests() {
		if r.Method == method && r.Path == path {
			out = append(out, r)
		}
	}
	return out
}

// TestAddonAccessResource_EmptySetRevokes covers what an absent block has to
// mean. Portainer leaves the stored policies alone when the field is
// omitted, so an authoritative resource must send an explicit empty object.
func TestAddonAccessResource_EmptySetRevokes(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	newFakeAddonAPI(mock)

	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{{
			Config: mockProviderConfig(mock) + addonAccessMinimalConfig,
			Check: resource.ComposeTestCheckFunc(
				resource.TestCheckResourceAttr("portainer_addon_access.a", "enabled", "true"),
				func(*terraform.State) error {
					var payload map[string]interface{}
					if err := mock.FindRequest("PUT", fakeAddonPath+"/access").DecodeJSON(&payload); err != nil {
						return fmt.Errorf("access payload is not JSON: %w", err)
					}
					for _, key := range []string{"userAccessPolicies", "teamAccessPolicies"} {
						policies, ok := payload[key].(map[string]interface{})
						if !ok {
							return fmt.Errorf("%s must be an object, got %T", key, payload[key])
						}
						if len(policies) != 0 {
							return fmt.Errorf("%s: expected an empty object to revoke every grant, got %v", key, policies)
						}
					}
					return nil
				},
			),
		}},
	})
}

// TestAddonAccessResource_ReadKeepsGrantsWithoutRecord guards the non-admin
// read: the policies live on the admin-only record, and a response that never
// carried them must not be taken as proof the grants are gone.
func TestAddonAccessResource_ReadKeepsGrantsWithoutRecord(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	fake := newFakeAddonAPI(mock)
	fake.noRecord = true

	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{
				Config: mockProviderConfig(mock) + addonAccessFullConfig,
				Check:  resource.TestCheckResourceAttr("portainer_addon_access.a", "user_access.#", "1"),
			},
			{
				Config:   mockProviderConfig(mock) + addonAccessFullConfig,
				PlanOnly: true,
			},
		},
	})
}

// TestAddonAccessResource_DriftDetected: a grant revoked outside Terraform
// plans its restoration.
func TestAddonAccessResource_DriftDetected(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	fake := newFakeAddonAPI(mock)

	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{Config: mockProviderConfig(mock) + addonAccessFullConfig},
			{
				PreConfig: func() {
					fake.mu.Lock()
					fake.users = map[string]map[string]int{}
					fake.mu.Unlock()
				},
				Config:             mockProviderConfig(mock) + addonAccessFullConfig,
				PlanOnly:           true,
				ExpectNonEmptyPlan: true,
			},
		},
	})
}

func TestAddonAccessResource_UpgradeFromV300(t *testing.T) {
	t.Parallel()
	for name, cfg := range map[string]string{"full": addonAccessFullConfig, "minimal": addonAccessMinimalConfig} {
		t.Run(name, func(t *testing.T) {
			mock := NewMockServer(t)
			newFakeAddonAPI(mock)
			addonUpgradeTest(t, mockProviderConfig(mock)+cfg)
		})
	}
}

// addonUpgradeTest applies config with v3.0.0 (SDKv2), then expects this
// build to plan no change against the state it wrote.
func addonUpgradeTest(t *testing.T, config string) {
	t.Helper()
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

const addonConfigInitial = `
resource "portainer_addon_config" "c" {
  addon_id = "portal-template"
  entry {
    key   = "BASE_DOMAIN"
    value = "old.example.com"
  }
  entry {
    key   = "KEEP"
    value = "same"
  }
  entry {
    key   = "OLD_KEY"
    value = "gone"
  }
  entry {
    key       = "API_TOKEN"
    value     = "s3cret"
    sensitive = true
  }
}
`

const addonConfigChanged = `
resource "portainer_addon_config" "c" {
  addon_id = "portal-template"
  entry {
    key   = "BASE_DOMAIN"
    value = "new.example.com"
  }
  entry {
    key   = "KEEP"
    value = "same"
  }
  entry {
    key       = "API_TOKEN"
    value     = "s3cret"
    sensitive = true
  }
}
`

// TestAddonConfigResource_Lifecycle: create replaces the whole configuration
// with a single PUT; an update PATCHes changed keys and DELETEs removed ones
// without touching an unchanged key (a key added outside Terraform would
// otherwise be destroyed on every apply); a withheld sensitive value keeps
// what was configured; destroy clears everything through the bulk endpoint.
func TestAddonConfigResource_Lifecycle(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	newFakeAddonAPI(mock)

	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		CheckDestroy: func(*terraform.State) error {
			if mock.FindRequest("DELETE", fakeAddonPath+"/config") == nil {
				return fmt.Errorf("destroy must clear the addon's configuration")
			}
			return nil
		},
		Steps: []resource.TestStep{
			{
				Config: mockProviderConfig(mock) + addonConfigInitial,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("portainer_addon_config.c", "entry.#", "4"),
					resource.TestCheckTypeSetElemNestedAttrs("portainer_addon_config.c", "entry.*", map[string]string{
						"key": "API_TOKEN", "value": "s3cret", "sensitive": "true",
					}),
					func(*terraform.State) error {
						var payload struct {
							Entries []addonConfigEntry `json:"entries"`
						}
						if err := mock.FindRequest("PUT", fakeAddonPath+"/config").DecodeJSON(&payload); err != nil {
							return fmt.Errorf("config payload is not JSON: %w", err)
						}
						if len(payload.Entries) != 4 || payload.Entries[0].Key != "API_TOKEN" || payload.Entries[1].Key != "BASE_DOMAIN" {
							return fmt.Errorf("expected the configured entries sorted by key, got %+v", payload.Entries)
						}
						return nil
					},
				),
			},
			{
				Config:   mockProviderConfig(mock) + addonConfigInitial,
				PlanOnly: true,
			},
			{
				Config: mockProviderConfig(mock) + addonConfigChanged,
				Check: func(*terraform.State) error {
					if mock.FindRequest("PATCH", fakeAddonPath+"/config/BASE_DOMAIN") == nil {
						return fmt.Errorf("a changed entry must be PATCHed")
					}
					if mock.FindRequest("DELETE", fakeAddonPath+"/config/OLD_KEY") == nil {
						return fmt.Errorf("a removed entry must be DELETEd")
					}
					for _, key := range []string{"KEEP", "API_TOKEN"} {
						if mock.FindRequest("PATCH", fakeAddonPath+"/config/"+key) != nil {
							return fmt.Errorf("an unchanged entry (%s) must not be rewritten", key)
						}
					}
					if n := len(addonRequests(mock, "PUT", fakeAddonPath+"/config")); n != 1 {
						return fmt.Errorf("update must not replace the whole configuration (%d PUTs)", n)
					}
					return nil
				},
			},
			{
				ResourceName:            "portainer_addon_config.c",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"entry"},
			},
		},
	})
}

func TestAddonConfigResource_EntryRequired(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{{
			Config:      mockProviderConfig(mock) + `resource "portainer_addon_config" "c" { addon_id = "portal-template" }`,
			ExpectError: regexp.MustCompile(`(?i)entry`),
		}},
	})
}

func TestAddonConfigResource_UpgradeFromV300(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	newFakeAddonAPI(mock)
	addonUpgradeTest(t, mockProviderConfig(mock)+addonConfigInitial)
}

// TestAccAddonAccessResource needs a Business Edition with an installed
// addon; there is no such instance in the acceptance environment.
func TestAccAddonAccessResource(t *testing.T) {
	if os.Getenv("PORTAINER_ACC_BE") != "1" || os.Getenv("PORTAINER_ACC_ADDON_ID") == "" {
		t.Skip("portainer_addon_access/portainer_addon_config need Portainer Business Edition with an installed addon; set PORTAINER_ACC_BE=1 and PORTAINER_ACC_ADDON_ID")
	}
	accPreCheck(t)
	resource.Test(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{{
			Config: accProvider() + fmt.Sprintf(`
resource "portainer_addon_access" "a" {
  addon_id = %q
}
`, os.Getenv("PORTAINER_ACC_ADDON_ID")),
			Check: resource.TestCheckResourceAttr("portainer_addon_access.a", "enabled", "true"),
		}},
	})
}
