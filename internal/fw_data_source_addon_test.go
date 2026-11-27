package internal

import (
	"fmt"
	"net/http"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// TestAddonsDataSource_ListsCatalogAndEnvironment covers the listing plus the
// environment lookup it folds in, and the catalog error that tells an admin
// the listing is a cached copy.
func TestAddonsDataSource_ListsCatalogAndEnvironment(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("GET", "/addons", RespondJSON(http.StatusOK, map[string]interface{}{
		"catalogError": "registry unreachable",
		"addons": []map[string]interface{}{
			{
				"id": "portal-template", "displayName": "Portal", "enabled": true,
				"lifecycleStatus": "installed", "healthStatus": "healthy",
				"record": map[string]interface{}{
					"chartVersion": "1.2.3", "upgradeAvailable": true,
					"availableVersions": []string{"1.3.0", "1.2.3"},
				},
			},
			// No record: this is what a non-admin sees, and it must not fail.
			{"id": "other", "displayName": "Other", "lifecycleStatus": ""},
		},
	}))
	mock.On("GET", "/addons/environment", RespondJSON(http.StatusOK, map[string]interface{}{"environmentId": 7}))

	const ds = "data.portainer_addons.all"
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{{
			Config: mockProviderConfig(mock) + `data "portainer_addons" "all" {}`,
			Check: resource.ComposeTestCheckFunc(
				resource.TestCheckResourceAttr(ds, "id", "portainer-addons"),
				resource.TestCheckResourceAttr(ds, "environment_id", "7"),
				resource.TestCheckResourceAttr(ds, "catalog_error", "registry unreachable"),
				resource.TestCheckResourceAttr(ds, "addons.#", "2"),
				resource.TestCheckResourceAttr(ds, "addons.0.chart_version", "1.2.3"),
				resource.TestCheckResourceAttr(ds, "addons.0.upgrade_available", "true"),
				resource.TestCheckResourceAttr(ds, "addons.0.available_versions.#", "2"),
				resource.TestCheckResourceAttr(ds, "addons.1.id", "other"),
				resource.TestCheckResourceAttr(ds, "addons.1.chart_version", ""),
				resource.TestCheckResourceAttr(ds, "addons.1.available_versions.#", "0"),
				func(*terraform.State) error {
					if req := mock.FindRequest("GET", "/addons"); req == nil || req.Query != "" {
						return fmt.Errorf("expected no query without a view")
					}
					return nil
				},
			),
		}},
	})
}

// TestAddonsDataSource_KubernetesRequired: Portainer not running on
// Kubernetes lists the catalog but answers 404 for /addons/environment; the
// error says add-ons need Portainer on Kubernetes.
func TestAddonsDataSource_KubernetesRequired(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("GET", "/addons", RespondJSON(http.StatusOK, map[string]interface{}{"addons": []map[string]interface{}{}}))
	mock.On("GET", "/addons/environment", RespondString(http.StatusNotFound, "text/plain", "404 page not found"))
	mock.On("GET", "/system/info", RespondJSON(http.StatusOK, map[string]interface{}{"platform": "Docker Standalone", "addonsAvailable": "kubernetes-required"}))
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{{
			Config:      mockProviderConfig(mock) + `data "portainer_addons" "all" {}`,
			ExpectError: regexp.MustCompile(`add-ons\s+need\s+Portainer\s+running\s+on\s+Kubernetes`),
		}},
	})
}

// TestAddonsDataSource_NotFoundWhileAvailable: a 404 on an instance where
// add-ons are available is reported as it is.
func TestAddonsDataSource_NotFoundWhileAvailable(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("GET", "/addons", RespondString(http.StatusNotFound, "text/plain", "404 page not found"))
	mock.On("GET", "/system/info", RespondJSON(http.StatusOK, map[string]interface{}{"addonsAvailable": "available"}))
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{{
			Config:      mockProviderConfig(mock) + `data "portainer_addons" "all" {}`,
			ExpectError: regexp.MustCompile(`failed to list the addons: GET \S+ failed with status 404`),
		}},
	})
}

// TestAddonsDataSource_SwitcherView pins the one query parameter the endpoint
// takes, which is also what makes the listing readable by a non-admin.
func TestAddonsDataSource_SwitcherView(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("GET", "/addons", RespondJSON(http.StatusOK, map[string]interface{}{"addons": []interface{}{}}))
	mock.On("GET", "/addons/environment", RespondJSON(http.StatusOK, map[string]interface{}{"environmentId": 1}))

	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{
				Config: mockProviderConfig(mock) + `data "portainer_addons" "all" { view = "switcher" }`,
				Check: func(*terraform.State) error {
					req := mock.FindRequest("GET", "/addons")
					if req == nil || !strings.Contains(req.Query, "view=switcher") {
						return fmt.Errorf("expected the switcher view in the query, got %v", req)
					}
					return nil
				},
			},
		},
	})

	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{{
			Config:      mockProviderConfig(mock) + `data "portainer_addons" "all" { view = "full" }`,
			ExpectError: regexp.MustCompile(`switcher`),
		}},
	})
}

// TestAddonChartSourceDataSource_QueriesRegistryAndChart pins the
// pre-install reachability check, whose whole point is the query it sends.
func TestAddonChartSourceDataSource_QueriesRegistryAndChart(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("GET", fakeAddonPath+"/chart-source", RespondJSON(http.StatusOK, map[string]interface{}{
		"reachable": true, "reason": "", "tlsVerify": true,
		"versions": []string{"1.3.0", "1.2.3"},
	}))

	const ds = "data.portainer_addon_chart_source.c"
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{{
			Config: mockProviderConfig(mock) + `
data "portainer_addon_chart_source" "c" {
  addon_id    = "portal-template"
  registry_id = 3
  chart       = "portainer/charts/portal"
}
`,
			Check: resource.ComposeTestCheckFunc(
				resource.TestCheckResourceAttr(ds, "id", "portainer-addon-chart-source-portal-template"),
				resource.TestCheckResourceAttr(ds, "reachable", "true"),
				resource.TestCheckResourceAttr(ds, "tls_verify", "true"),
				resource.TestCheckResourceAttr(ds, "versions.#", "2"),
				resource.TestCheckResourceAttr(ds, "versions.0", "1.3.0"),
				func(*terraform.State) error {
					req := mock.FindRequest("GET", fakeAddonPath+"/chart-source")
					if req == nil {
						return fmt.Errorf("no chart-source request was sent")
					}
					if !strings.Contains(req.Query, "registryId=3") {
						return fmt.Errorf("expected registryId in the query, got %q", req.Query)
					}
					if !strings.Contains(req.Query, "chart=portainer%2Fcharts%2Fportal") {
						return fmt.Errorf("expected the chart path escaped in the query, got %q", req.Query)
					}
					return nil
				},
			),
		}},
	})
}

func TestAddonCatalogDataSource(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("GET", "/addons/catalog", RespondJSON(http.StatusOK, map[string]interface{}{
		"addons": []map[string]interface{}{{
			"id": "portal-template", "displayName": "Portal", "serviceUrl": "http://portal:8080",
			"healthCheckPath": "/readyz", "helmChartRepository": "oci://ghcr.io/portainer/charts/portal-template",
			"tlsVerify": true, "machineAccess": []string{"deploy"},
		}},
	}))

	const ds = "data.portainer_addon_catalog.c"
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{{
			Config: mockProviderConfig(mock) + `data "portainer_addon_catalog" "c" {}`,
			Check: resource.ComposeTestCheckFunc(
				resource.TestCheckResourceAttr(ds, "id", "portainer-addon-catalog"),
				resource.TestCheckResourceAttr(ds, "addons.0.id", "portal-template"),
				resource.TestCheckResourceAttr(ds, "addons.0.display_name", "Portal"),
				resource.TestCheckResourceAttr(ds, "addons.0.service_url", "http://portal:8080"),
				resource.TestCheckResourceAttr(ds, "addons.0.health_check_path", "/readyz"),
				resource.TestCheckResourceAttr(ds, "addons.0.tls_verify", "true"),
				resource.TestCheckResourceAttr(ds, "addons.0.machine_access.0", "deploy"),
			),
		}},
	})
}

// TestAddonCatalogDataSource_NoCopy verifies the 503 Portainer answers when it
// has no catalog at all fails the read instead of reporting an empty catalog.
func TestAddonCatalogDataSource_NoCopy(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("GET", "/addons/catalog", RespondString(http.StatusServiceUnavailable, "", "catalog unavailable"))

	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{{
			Config:      mockProviderConfig(mock) + `data "portainer_addon_catalog" "c" {}`,
			ExpectError: regexp.MustCompile(`503`),
		}},
	})
}

// TestAddonCatalog_ExplainsUnsupported: the add-on catalog (BE 2.45.2+) names
// its requirement on a server without it instead of failing with a bare 404.
func TestAddonCatalog_ExplainsUnsupported(t *testing.T) {
	t.Parallel()
	cases := map[string]struct{ method, path, version, edition, config string }{
		"catalog":         {"GET", "/addons/catalog", "2.45.1", "EE", `data "portainer_addon_catalog" "c" {}`},
		"catalog refresh": {"POST", "/addons/catalog/refresh", "2.45.3", "CE", `resource "portainer_addon_catalog_refresh" "r" {}`},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			mock := NewMockServer(t)
			mock.On(tc.method, tc.path, RespondString(http.StatusNotFound, "text/plain", "404 page not found"))
			mock.On("GET", "/system/version", RespondJSON(http.StatusOK, map[string]interface{}{
				"ServerVersion": tc.version, "ServerEdition": tc.edition,
			}))
			resource.UnitTest(t, resource.TestCase{
				ProtoV5ProviderFactories: testProtoV5Factories,
				Steps: []resource.TestStep{{
					Config:      mockProviderConfig(mock) + tc.config,
					ExpectError: regexp.MustCompile(`Business Edition 2\.45\.2 or\s+later`),
				}},
			})
		})
	}
}

// TestAddonCatalogRefreshResource posts the refresh, records the run, and
// replaces the resource when a trigger changes; a 403 fails the apply.
func TestAddonCatalogRefreshResource(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("POST", "/addons/catalog/refresh", RespondString(http.StatusNoContent, "", ""))

	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{
				Config: mockProviderConfig(mock) + `resource "portainer_addon_catalog_refresh" "r" {}`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestMatchResourceAttr("portainer_addon_catalog_refresh.r", "id", regexp.MustCompile(`^addon-catalog-refresh-\d+$`)),
					func(*terraform.State) error {
						if mock.FindRequest("POST", "/addons/catalog/refresh") == nil {
							return fmt.Errorf("expected POST /addons/catalog/refresh")
						}
						return nil
					},
				),
			},
			{
				Config: mockProviderConfig(mock) + `resource "portainer_addon_catalog_refresh" "r" { triggers = { url = "a" } }`,
				Check: func(*terraform.State) error {
					if n := len(addonRequests(mock, "POST", "/addons/catalog/refresh")); n != 2 {
						return fmt.Errorf("a trigger change must refresh again, got %d refreshes", n)
					}
					return nil
				},
			},
		},
	})

	forbidden := NewMockServer(t)
	forbidden.On("POST", "/addons/catalog/refresh", RespondString(http.StatusForbidden, "", "admins only"))
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{{
			Config:      mockProviderConfig(forbidden) + `resource "portainer_addon_catalog_refresh" "r" {}`,
			ExpectError: regexp.MustCompile(`403`),
		}},
	})
}

// TestAddonRepairResource_PostsAndRecordsOutcome covers the repair action and
// the lifecycle fields it reports back.
func TestAddonRepairResource_PostsAndRecordsOutcome(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("POST", fakeAddonPath+"/repair", RespondJSON(http.StatusOK, map[string]interface{}{
		"id": "portal-template", "enabled": true, "chartVersion": "1.2.3",
		"lifecycleStatus": "installed", "lifecycleStatusMessage": "",
	}))

	const r = "portainer_addon_repair.r"
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{
				Config: mockProviderConfig(mock) + `resource "portainer_addon_repair" "r" { addon_id = "portal-template" }`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(r, "lifecycle_status", "installed"),
					resource.TestCheckResourceAttr(r, "chart_version", "1.2.3"),
					resource.TestCheckResourceAttr(r, "enabled", "true"),
					resource.TestMatchResourceAttr(r, "id", regexp.MustCompile(`^portal-template-repair-\d+$`)),
				),
			},
			{
				Config:   mockProviderConfig(mock) + `resource "portainer_addon_repair" "r" { addon_id = "portal-template" }`,
				PlanOnly: true,
			},
		},
	})
}

func TestAddonRepairResource_UpgradeFromV300(t *testing.T) {
	t.Parallel()
	for name, cfg := range map[string]string{
		"no triggers": `resource "portainer_addon_repair" "r" { addon_id = "portal-template" }`,
		"triggers": `
resource "portainer_addon_repair" "r" {
  addon_id = "portal-template"
  triggers = { v = "1" }
}
`,
	} {
		t.Run(name, func(t *testing.T) {
			mock := NewMockServer(t)
			mock.On("POST", fakeAddonPath+"/repair", RespondJSON(http.StatusOK, map[string]interface{}{
				"id": "portal-template", "enabled": true, "chartVersion": "1.2.3", "lifecycleStatus": "installed",
			}))
			addonUpgradeTest(t, mockProviderConfig(mock)+cfg)
		})
	}
}

func TestAddonCatalogRefreshResource_UpgradeFromV300(t *testing.T) {
	t.Parallel()
	for name, cfg := range map[string]string{
		"no triggers": `resource "portainer_addon_catalog_refresh" "r" {}`,
		"triggers":    `resource "portainer_addon_catalog_refresh" "r" { triggers = { url = "a" } }`,
	} {
		t.Run(name, func(t *testing.T) {
			mock := NewMockServer(t)
			mock.On("POST", "/addons/catalog/refresh", RespondString(http.StatusNoContent, "", ""))
			addonUpgradeTest(t, mockProviderConfig(mock)+cfg)
		})
	}
}

// TestAccAddonCatalogDataSource needs Business Edition 2.45.2 or later.
func TestAccAddonCatalogDataSource(t *testing.T) {
	if os.Getenv("PORTAINER_ACC_BE") != "1" {
		t.Skip("portainer_addon_catalog/portainer_addon_catalog_refresh/portainer_addon_repair need Portainer Business Edition; set PORTAINER_ACC_BE=1")
	}
	accPreCheck(t)
	resource.Test(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{{
			Config: accProvider() + `data "portainer_addon_catalog" "c" {}`,
			Check:  resource.TestCheckResourceAttr("data.portainer_addon_catalog.c", "id", "portainer-addon-catalog"),
		}},
	})
}
