package internal

import (
	"net/http"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func dataSourceTest(t *testing.T, mock *MockServer, config string, check resource.TestCheckFunc) {
	t.Helper()
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps:                    []resource.TestStep{{Config: mockProviderConfig(mock) + config, Check: check}},
	})
}

func dataSourceErrorTest(t *testing.T, mock *MockServer, config, want string) {
	t.Helper()
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps:                    []resource.TestStep{{Config: mockProviderConfig(mock) + config, ExpectError: regexp.MustCompile(want)}},
	})
}

func mockSystemEndpoints(t *testing.T, adminStatus int, addons interface{}) *MockServer {
	t.Helper()
	mock := NewMockServer(t)
	mock.On("GET", "/system/status", RespondJSON(http.StatusOK, map[string]interface{}{
		"InstanceID": "abc-123", "Version": "2.45.0",
	}))
	mock.On("GET", "/system/version", RespondJSON(http.StatusOK, map[string]interface{}{
		"ServerVersion": "2.45.0", "ServerEdition": "CE", "DatabaseVersion": "150",
		"LatestVersion": "2.46.0", "UpdateAvailable": true, "VersionSupport": "LTS",
	}))
	info := map[string]interface{}{"agents": 3, "edgeAgents": 7, "platform": "docker"}
	if addons != nil {
		info["addonsAvailable"] = addons
	}
	mock.On("GET", "/system/info", RespondJSON(http.StatusOK, info))
	mock.On("GET", "/system/nodes", RespondJSON(http.StatusOK, map[string]interface{}{"nodes": 12}))
	if adminStatus == http.StatusNoContent {
		mock.On("GET", "/users/admin/check", RespondString(http.StatusNoContent, "", ""))
	} else {
		mock.On("GET", "/users/admin/check", RespondString(adminStatus, "application/json", `{"message":"no admin"}`))
	}
	return mock
}

// TestSystemDataSource folds the four system endpoints into one data source;
// the admin check's 404 reads as "not initialised" while a 500 fails.
func TestSystemDataSource(t *testing.T) {
	t.Parallel()
	const ds = "data.portainer_system.s"
	config := `data "portainer_system" "s" {}`
	t.Run("happy", func(t *testing.T) {
		checks := make([]resource.TestCheckFunc, 0, 16)
		checks = append(checks, resource.TestCheckResourceAttr(ds, "id", "portainer-system-abc-123"))
		for k, v := range map[string]string{
			"instance_id": "abc-123", "version": "2.45.0", "server_version": "2.45.0", "server_edition": "CE",
			"database_version": "150", "latest_version": "2.46.0", "update_available": "true", "version_support": "LTS",
			"agents": "3", "edge_agents": "7", "platform": "docker", "nodes": "12", "admin_initialized": "true",
			"addons_available": "false", "addons_availability": "",
		} {
			checks = append(checks, resource.TestCheckResourceAttr(ds, k, v))
		}
		dataSourceTest(t, mockSystemEndpoints(t, http.StatusNoContent, nil), config, resource.ComposeTestCheckFunc(checks...))
	})
	t.Run("uninitialised", func(t *testing.T) {
		dataSourceTest(t, mockSystemEndpoints(t, http.StatusNotFound, nil), config, resource.TestCheckResourceAttr(ds, "admin_initialized", "false"))
	})
	t.Run("admin check error", func(t *testing.T) {
		dataSourceErrorTest(t, mockSystemEndpoints(t, http.StatusInternalServerError, nil), config, `failed to check for an administrator account`)
	})
	// A 2.45 (bool) and a 3.0 (string) system/info.
	for name, tc := range map[string]struct {
		value                   interface{}
		available, availability string
	}{
		"2.45 true":   {true, "true", "available"},
		"2.45 false":  {false, "false", "unavailable"},
		"3.0 ok":      {"available", "true", "available"},
		"3.0 not k8s": {"kubernetes-required", "false", "kubernetes-required"},
	} {
		t.Run(name, func(t *testing.T) {
			dataSourceTest(t, mockSystemEndpoints(t, http.StatusNoContent, tc.value), config, resource.ComposeTestCheckFunc(
				resource.TestCheckResourceAttr(ds, "addons_available", tc.available),
				resource.TestCheckResourceAttr(ds, "addons_availability", tc.availability),
			))
		})
	}
}

// TestSettingsPublicDataSource covers the unauthenticated settings endpoint
// used to discover how an instance expects to be authenticated.
func TestSettingsPublicDataSource(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("GET", "/settings/public", RespondJSON(http.StatusOK, map[string]interface{}{
		"AuthenticationMethod": 3, "EnableEdgeComputeFeatures": true,
		"RequiredPasswordLength": 12, "RequiresSetupToken": false,
		"KubeconfigExpiry": "24h", "OAuthLoginURI": "https://sso.example.com/auth",
		"TeamSync": true,
	}))
	const ds = "data.portainer_settings_public.p"
	dataSourceTest(t, mock, `data "portainer_settings_public" "p" {}`, resource.ComposeTestCheckFunc(
		resource.TestCheckResourceAttr(ds, "id", "portainer-public-settings"),
		resource.TestCheckResourceAttr(ds, "authentication_method", "3"),
		resource.TestCheckResourceAttr(ds, "enable_edge_compute_features", "true"),
		resource.TestCheckResourceAttr(ds, "required_password_length", "12"),
		resource.TestCheckResourceAttr(ds, "kubeconfig_expiry", "24h"),
		resource.TestCheckResourceAttr(ds, "oauth_login_uri", "https://sso.example.com/auth"),
		resource.TestCheckResourceAttr(ds, "logo_url", ""),
		resource.TestCheckResourceAttr(ds, "team_sync", "true"),
	))
}

// TestMOTDDataSource: Hash is []byte upstream and arrives base64-encoded.
func TestMOTDDataSource(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("GET", "/motd", RespondJSON(http.StatusOK, map[string]interface{}{
		"Title": "Maintenance", "Message": "Upgrading on Sunday", "Style": "color: red", "Hash": []byte("abc"),
	}))
	const ds = "data.portainer_motd.m"
	dataSourceTest(t, mock, `data "portainer_motd" "m" {}`, resource.ComposeTestCheckFunc(
		resource.TestCheckResourceAttr(ds, "id", "portainer-motd"),
		resource.TestCheckResourceAttr(ds, "title", "Maintenance"),
		resource.TestCheckResourceAttr(ds, "style", "color: red"),
		resource.TestCheckResourceAttr(ds, "hash", "YWJj"),
	))
}

// TestLicensesInfoDataSource covers the field worth alerting on: a non-zero
// overuse timestamp means the instance is over its node count.
func TestLicensesInfoDataSource(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("GET", "/licenses/info", RespondJSON(http.StatusOK, map[string]interface{}{
		"valid": true, "type": 2, "company": "Example Ltd", "nodes": 50,
		"expiresAt": 1800000000, "enforcedAt": 0, "overuseStartedTimestamp": 1756000000,
	}))
	const ds = "data.portainer_licenses_info.l"
	dataSourceTest(t, mock, `data "portainer_licenses_info" "l" {}`, resource.ComposeTestCheckFunc(
		resource.TestCheckResourceAttr(ds, "id", "portainer-licenses-info"),
		resource.TestCheckResourceAttr(ds, "nodes", "50"),
		resource.TestCheckResourceAttr(ds, "company", "Example Ltd"),
		resource.TestCheckResourceAttr(ds, "expires_at", "1800000000"),
		resource.TestCheckResourceAttr(ds, "overuse_started_at", "1756000000"),
		resource.TestCheckResourceAttr(ds, "enforced_at", "0"),
	))
}

// TestCustomTemplateDataSource looks a template up by title in the list.
func TestCustomTemplateDataSource(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("GET", "/custom_templates", RespondJSON(http.StatusOK, []map[string]interface{}{
		{"Id": 4, "Title": "other", "Type": 1},
		{"Id": 5, "Title": "web", "Description": "Web stack", "Type": 2},
	}))
	const ds = "data.portainer_custom_template.t"
	dataSourceTest(t, mock, `data "portainer_custom_template" "t" { title = "web" }`, resource.ComposeTestCheckFunc(
		resource.TestCheckResourceAttr(ds, "id", "5"),
		resource.TestCheckResourceAttr(ds, "description", "Web stack"),
		resource.TestCheckResourceAttr(ds, "type", "2"),
	))
	dataSourceErrorTest(t, mock, `data "portainer_custom_template" "t" { title = "missing" }`, `custom template missing not found`)

	failing := NewMockServer(t)
	failing.On("GET", "/custom_templates", RespondString(http.StatusInternalServerError, "application/json", `{"message":"boom"}`))
	dataSourceErrorTest(t, failing, `data "portainer_custom_template" "t" { title = "web" }`, `failed to list custom templates`)
}

// TestCustomTemplateFileDataSource reads the stored file (FileContent).
func TestCustomTemplateFileDataSource(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("GET", "/custom_templates/3/file", RespondJSON(http.StatusOK, map[string]interface{}{"FileContent": "custom"}))
	const ds = "data.portainer_custom_template_file.f"
	dataSourceTest(t, mock, `data "portainer_custom_template_file" "f" { template_id = 3 }`, resource.ComposeTestCheckFunc(
		resource.TestCheckResourceAttr(ds, "id", "3/file"),
		resource.TestCheckResourceAttr(ds, "file_content", "custom"),
	))
	dataSourceErrorTest(t, mock, `data "portainer_custom_template_file" "f" { template_id = 4 }`, `failed to read the file of custom template 4`)
}

// TestCloudCredentialsDataSource looks credentials up by name.
func TestCloudCredentialsDataSource(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("GET", "/cloud/credentials", RespondJSON(http.StatusOK, []map[string]interface{}{
		{"id": 2, "name": "prod-aws", "provider": "aws"},
		{"id": 3, "name": "do", "provider": "digitalocean"},
	}))
	const ds = "data.portainer_cloud_credentials.c"
	dataSourceTest(t, mock, `data "portainer_cloud_credentials" "c" { name = "do" }`, resource.ComposeTestCheckFunc(
		resource.TestCheckResourceAttr(ds, "id", "3"),
		resource.TestCheckResourceAttr(ds, "cloud_provider", "digitalocean"),
	))
	dataSourceErrorTest(t, mock, `data "portainer_cloud_credentials" "c" { name = "gcp" }`, `cloud credentials gcp not found`)
}

// TestAccSystemDataSources reads the read-only endpoints of a real (CE)
// Portainer.
func TestAccSystemDataSources(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { accPreCheck(t) },
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{{
			Config: accProvider() + `
data "portainer_system" "s" {}
data "portainer_settings_public" "p" {}
data "portainer_motd" "m" {}
`,
			Check: resource.ComposeTestCheckFunc(
				resource.TestCheckResourceAttrSet("data.portainer_system.s", "server_version"),
				resource.TestCheckResourceAttr("data.portainer_system.s", "admin_initialized", "true"),
				resource.TestCheckResourceAttrSet("data.portainer_settings_public.p", "authentication_method"),
				resource.TestCheckResourceAttr("data.portainer_motd.m", "id", "portainer-motd"),
			),
		}},
	})
}
