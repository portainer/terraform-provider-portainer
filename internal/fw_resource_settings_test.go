package internal

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// fakePortainerSettings is GET/PUT /settings. Top-level keys are matched
// without regard to case (PUT takes camelCase, GET answers PascalCase), and a
// PUT replaces each object it sends, as Portainer does.
type fakePortainerSettings struct {
	mu       sync.Mutex
	settings map[string]interface{}
}

// portainerDefaultSettings is what an unconfigured Portainer reports.
func portainerDefaultSettings() map[string]interface{} {
	return map[string]interface{}{
		"authenticationmethod": 1,
		"blacklistedlabels":    []interface{}{},
		"internalauthsettings": map[string]interface{}{"RequiredPasswordLength": 12},
		"ldapsettings": map[string]interface{}{
			"AnonymousMode": true, "AutoCreateUsers": true, "TLSConfig": map[string]interface{}{},
			"SearchSettings": []interface{}{map[string]interface{}{}}, "GroupSearchSettings": []interface{}{map[string]interface{}{}},
		},
		"oauthsettings":            map[string]interface{}{"SSO": true},
		"snapshotinterval":         "5m",
		"edgeagentcheckininterval": 5,
		"usersessiontimeout":       "8h",
		"kubeconfigexpiry":         "0",
		"kubectlshellimage":        "portainer/kubectl-shell",
	}
}

func newFakePortainerSettings(m *MockServer, initial map[string]interface{}) *fakePortainerSettings {
	f := &fakePortainerSettings{settings: initial}
	if f.settings == nil {
		f.settings = map[string]interface{}{}
	}
	m.On("PUT", "/settings", func(w http.ResponseWriter, r *http.Request) {
		var body map[string]interface{}
		_ = json.NewDecoder(r.Body).Decode(&body)
		f.mu.Lock()
		for k, v := range body {
			// The OIDC provider settings are top-level fields of PUT and
			// one object (OIDCProviderSettings) in GET.
			if name, ok := strings.CutPrefix(k, "OIDCProvider"); ok {
				oidc, _ := f.settings["oidcprovidersettings"].(map[string]interface{})
				if oidc == nil {
					oidc = map[string]interface{}{}
				}
				oidc[name] = v
				f.settings["oidcprovidersettings"] = oidc
				continue
			}
			f.settings[strings.ToLower(k)] = v
		}
		f.mu.Unlock()
		RespondJSON(http.StatusOK, map[string]interface{}{})(w, r)
	})
	m.On("GET", "/settings", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		out := map[string]interface{}{}
		for k, v := range f.settings {
			switch k {
			case "ldapsettings":
				k = "LDAPSettings"
				// Portainer reports every field of the LDAP settings.
				full := map[string]interface{}{
					"URLs": nil, "AdminGroups": nil, "ServerType": 0, "AdminAutoPopulate": false, "AdminGroupSearchSettings": nil,
					"AnonymousMode": false, "AutoCreateUsers": false, "ReaderDN": "", "StartTLS": false, "URL": "",
				}
				for fk, fv := range v.(map[string]interface{}) {
					full[fk] = fv
				}
				v = full
			case "oauthsettings":
				k = "OAuthSettings"
			}
			out[k] = v
		}
		RespondJSON(http.StatusOK, out)(w, r)
	})
	return f
}

func (f *fakePortainerSettings) set(k string, v interface{}) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.settings[k] = v
}

const settingsFullConfig = `
resource "portainer_settings" "s" {
  authentication_method        = 3
  logo_url                     = "https://example.com/logo.png"
  snapshot_interval            = "15m"
  enable_edge_compute_features = true
  edge_agent_checkin_interval  = 10
  user_session_timeout         = "4h"

  black_listed_labels {
    name  = "hidden"
    value = "true"
  }
  global_deployment_options {
    hide_stacks_functionality = true
  }
  internal_auth_settings {
    required_password_length = 16
  }
  oauth_settings {
    client_id         = "portainer"
    client_secret     = "oauth-secret"
    authorization_uri = "https://sso.example.com/auth"
    access_token_uri  = "https://sso.example.com/token"
    resource_uri      = "https://sso.example.com/userinfo"
    redirect_uri      = "https://portainer.example.com"
    scopes            = "openid"
    user_identifier   = "email"
    sso               = true
    team_memberships {
      oauth_claim_name              = "groups"
      admin_group_claims_regex_list = ["^admins$"]
      oauth_claim_mappings {
        claim_val_regex = "^dev$"
        team            = 2
      }
    }
  }
  ldap_settings {
    url       = "ldap.example.com:389"
    reader_dn = "cn=reader,dc=example,dc=com"
    password  = "ldap-secret"
    search_settings {
      base_dn             = "ou=people,dc=example,dc=com"
      user_name_attribute = "uid"
    }
    tls_config {
      tls = true
    }
  }
}`

// TestSettingsResource_Lifecycle: every block reaches PUT /settings, the
// secrets are kept in state, the next plan is empty, an update, drift, and
// import.
func TestSettingsResource_Lifecycle(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	fake := newFakePortainerSettings(mock, portainerDefaultSettings())
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{
				Config: mockProviderConfig(mock) + settingsFullConfig,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("portainer_settings.s", "id", "portainer-settings"),
					resource.TestCheckResourceAttr("portainer_settings.s", "kubeconfig_expiry", "0"),
					resource.TestCheckResourceAttr("portainer_settings.s", "oauth_settings.0.client_secret", "oauth-secret"),
					resource.TestCheckResourceAttr("portainer_settings.s", "oauth_settings.0.team_memberships.0.oauth_claim_mappings.0.team", "2"),
					resource.TestCheckResourceAttr("portainer_settings.s", "ldap_settings.0.password", "ldap-secret"),
					resource.TestCheckResourceAttr("portainer_settings.s", "ldap_settings.0.anonymous_mode", "false"),
					func(*terraform.State) error {
						p := gsbBody(t, mock, "PUT", "/settings")
						if p["authenticationMethod"] != float64(3) || p["snapshotInterval"] != "15m" || p["enableEdgeComputeFeatures"] != true {
							return fmt.Errorf("top-level payload mismatch: %v", p)
						}
						oauth := p["oauthSettings"].(map[string]interface{})
						if oauth["ClientID"] != "portainer" || oauth["ClientSecret"] != "oauth-secret" {
							return fmt.Errorf("oauthSettings mismatch: %v", oauth)
						}
						tm := oauth["TeamMemberships"].(map[string]interface{})
						if tm["OAuthClaimName"] != "groups" || len(tm["OAuthClaimMappings"].([]interface{})) != 1 {
							return fmt.Errorf("TeamMemberships mismatch: %v", tm)
						}
						ldap := p["ldapsettings"].(map[string]interface{})
						if ldap["URL"] != "ldap.example.com:389" || ldap["Password"] != "ldap-secret" || ldap["TLSConfig"].(map[string]interface{})["TLS"] != true {
							return fmt.Errorf("ldapsettings mismatch: %v", ldap)
						}
						if p["internalAuthSettings"].(map[string]interface{})["requiredPasswordLength"] != float64(16) {
							return fmt.Errorf("internalAuthSettings mismatch: %v", p["internalAuthSettings"])
						}
						if fmt.Sprint(p["blackListedLabels"]) != "[map[name:hidden value:true]]" {
							return fmt.Errorf("blackListedLabels mismatch: %v", p["blackListedLabels"])
						}
						return nil
					},
				),
			},
			{
				Config: mockProviderConfig(mock) + strings.Replace(settingsFullConfig, `"15m"`, `"30m"`, 1),
				Check: func(*terraform.State) error {
					if p := gsbBody(t, mock, "PUT", "/settings"); p["snapshotInterval"] != "30m" {
						return fmt.Errorf("snapshotInterval: got %v", p["snapshotInterval"])
					}
					return nil
				},
			},
			{
				PreConfig:          func() { fake.set("logourl", "https://elsewhere.example.com/logo.png") },
				Config:             mockProviderConfig(mock) + strings.Replace(settingsFullConfig, `"15m"`, `"30m"`, 1),
				PlanOnly:           true,
				ExpectNonEmptyPlan: true,
			},
			{Config: mockProviderConfig(mock) + strings.Replace(settingsFullConfig, `"15m"`, `"30m"`, 1)},
			{
				ResourceName:            "portainer_settings.s",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"oauth_settings.0.client_secret", "ldap_settings.0.password"},
			},
		},
	})
}

// TestSettingsResource_UnconfiguredBlocksStayOut: Portainer reports LDAP,
// OAuth and password settings whether they are configured or not; a
// configuration without those blocks plans no change.
func TestSettingsResource_UnconfiguredBlocksStayOut(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	newFakePortainerSettings(mock, portainerDefaultSettings())
	config := mockProviderConfig(mock) + `resource "portainer_settings" "s" { enable_edge_compute_features = true }`
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{
				Config: config,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("portainer_settings.s", "ldap_settings.#", "0"),
					resource.TestCheckResourceAttr("portainer_settings.s", "snapshot_interval", "5m"),
					func(*terraform.State) error {
						p := gsbBody(t, mock, "PUT", "/settings")
						for _, k := range []string{"ldapsettings", "oauthSettings", "internalAuthSettings", "AddonsCatalogURL"} {
							if _, ok := p[k]; ok {
								return fmt.Errorf("%s must be left out when not configured", k)
							}
						}
						return nil
					},
				),
			},
			{RefreshState: true, Check: resource.TestCheckResourceAttr("portainer_settings.s", "oauth_settings.#", "0")},
			{Config: config, PlanOnly: true},
		},
	})
}

// TestSettingsResource_ConfiguredSwitches is the regression test for switches
// that could be turned on but never off: false is sent when configured, and
// a switch the configuration leaves out is not sent, so Portainer keeps its
// own value. force_secure_cookies is read back.
func TestSettingsResource_ConfiguredSwitches(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	newFakePortainerSettings(mock, nil)
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{{
			Config: mockProviderConfig(mock) + `
resource "portainer_settings" "s" {
  enable_edge_compute_features = false
  disable_kube_shell           = false
  force_secure_cookies         = true
  addons_catalog_url           = "https://charts.example.com/addons.json"
}`,
			Check: resource.ComposeTestCheckFunc(
				resource.TestCheckResourceAttr("portainer_settings.s", "force_secure_cookies", "true"),
				func(*terraform.State) error {
					p := gsbBody(t, mock, "PUT", "/settings")
					for _, key := range []string{"enableEdgeComputeFeatures", "DisableKubeShell"} {
						if got, ok := p[key]; !ok || got != false {
							return fmt.Errorf("payload.%s: expected the configured false, got %v (present=%v)", key, got, ok)
						}
					}
					for _, key := range []string{"trustOnFirstConnect", "enforceEdgeID"} {
						if _, ok := p[key]; ok {
							return fmt.Errorf("payload.%s: expected it left out when not configured", key)
						}
					}
					if p["ForceSecureCookies"] != true || p["AddonsCatalogURL"] != "https://charts.example.com/addons.json" {
						return fmt.Errorf("payload mismatch: %v", p)
					}
					return nil
				},
			),
		}},
	})
}

// TestSettingsResource_WriteOnlySecrets: ldap_password_wo and
// oauth_client_secret_wo reach Portainer, and need their blocks.
func TestSettingsResource_WriteOnlySecrets(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	newFakePortainerSettings(mock, nil)
	config := mockProviderConfig(mock) + `
resource "portainer_settings" "s" {
  ldap_password_wo               = "ldap-secret"
  ldap_password_wo_version       = 1
  oauth_client_secret_wo         = "oauth-secret"
  oauth_client_secret_wo_version = 1
  ldap_settings {
    reader_dn = "cn=reader"
    url       = "ldap.example.com:389"
  }
  oauth_settings {
    client_id = "portainer"
  }
}`
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{
				Config: config,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("portainer_settings.s", "ldap_settings.0.password", ""),
					func(*terraform.State) error {
						p := gsbBody(t, mock, "PUT", "/settings")
						if p["ldapsettings"].(map[string]interface{})["Password"] != "ldap-secret" {
							return fmt.Errorf("ldapsettings.Password: expected the write-only value")
						}
						if p["oauthSettings"].(map[string]interface{})["ClientSecret"] != "oauth-secret" {
							return fmt.Errorf("oauthSettings.ClientSecret: expected the write-only value")
						}
						return nil
					},
				),
			},
			{Config: config, PlanOnly: true},
		},
	})
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{{
			Config: mockProviderConfig(mock) + `
resource "portainer_settings" "s" {
  ldap_password_wo         = "ldap-secret"
  ldap_password_wo_version = 1
}`,
			ExpectError: regexp.MustCompile(`ldap_password_wo,ldap_password_wo_version,ldap_settings`),
		}},
	})
}

func TestSettingsResource_HTTPErrors(t *testing.T) {
	t.Parallel()
	put := NewMockServer(t)
	put.On("PUT", "/settings", RespondString(http.StatusBadRequest, "application/json", `{"message":"bad"}`))
	get := NewMockServer(t)
	get.On("PUT", "/settings", RespondJSON(http.StatusOK, map[string]interface{}{}))
	get.On("GET", "/settings", RespondString(http.StatusInternalServerError, "application/json", `{"message":"boom"}`))
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{Config: mockProviderConfig(put) + `resource "portainer_settings" "s" { logo_url = "x" }`, ExpectError: regexp.MustCompile(`failed to update settings`)},
			{Config: mockProviderConfig(get) + `resource "portainer_settings" "s" { logo_url = "x" }`, ExpectError: regexp.MustCompile(`failed to read settings`)},
		},
	})
}

func TestSettingsResource_UpgradeFromV300(t *testing.T) {
	t.Parallel()
	newMock := func(t *testing.T) *MockServer {
		m := NewMockServer(t)
		newFakePortainerSettings(m, portainerDefaultSettings())
		return m
	}
	gsbUpgradeTest(t, newMock,
		`resource "portainer_settings" "s" { enable_edge_compute_features = true }`,
		`
resource "portainer_settings" "s" {
  authentication_method = 1
  logo_url              = "https://example.com/logo.png"
  snapshot_interval     = "5m"
  user_session_timeout  = "8h"

  internal_auth_settings {
    required_password_length = 14
  }

  ldap_settings {
    anonymous_mode    = false
    auto_create_users = true
    password          = "p"
    reader_dn         = "cn=reader,dc=example,dc=com"
    start_tls         = false
    url               = "ldap.example.com:389"
  }
}`,
		settingsFullConfig,
		`
resource "portainer_settings" "s" {
  disable_kube_roles_sync     = false
  disable_kube_shell          = false
  disable_kubeconfig_download = false
}`,
	)
}

// TestAccSettingsResource reads and rewrites a real (CE) Portainer's
// settings with the values it already has.
func TestAccSettingsResource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { accPreCheck(t) },
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{
				Config: accProvider() + `resource "portainer_settings" "s" { enable_edge_compute_features = true }`,
				Check:  resource.TestCheckResourceAttr("portainer_settings.s", "enable_edge_compute_features", "true"),
			},
			{Config: accProvider() + `resource "portainer_settings" "s" { enable_edge_compute_features = true }`, PlanOnly: true},
		},
	})
}

// TestSettingsResource_BusinessEditionFields: custom_login_banner and the
// auto_patch_settings and edge blocks reach PUT /settings with only their
// known values, are read back (the mTLS certificates keep their configured
// value), drift shows, and a configuration without them sends none of them.
func TestSettingsResource_BusinessEditionFields(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	fake := newFakePortainerSettings(mock, portainerDefaultSettings())
	config := mockProviderConfig(mock) + `
resource "portainer_settings" "s" {
  custom_login_banner = "Authorised use only"

  auto_patch_settings {
    enabled    = true
    patch_cron = "0 4 * * *"
  }

  edge {
    ping_interval         = 10
    tunnel_server_address = "portainer.example.com:8000"
    mtls {
      use_separate_cert = true
      ca_cert           = "CA"
      cert              = "CERT"
      key               = "KEY"
    }
  }
}`
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{
				Config: config,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("portainer_settings.s", "custom_login_banner", "Authorised use only"),
					resource.TestCheckResourceAttr("portainer_settings.s", "auto_patch_settings.0.patch_cron", "0 4 * * *"),
					resource.TestCheckResourceAttr("portainer_settings.s", "auto_patch_settings.0.portainer_repository", ""),
					resource.TestCheckResourceAttr("portainer_settings.s", "edge.0.ping_interval", "10"),
					resource.TestCheckResourceAttr("portainer_settings.s", "edge.0.command_interval", "0"),
					resource.TestCheckResourceAttr("portainer_settings.s", "edge.0.mtls.0.use_separate_cert", "true"),
					resource.TestCheckResourceAttr("portainer_settings.s", "edge.0.mtls.0.key", "KEY"),
					func(*terraform.State) error {
						p := gsbBody(t, mock, "PUT", "/settings")
						if p["CustomLoginBanner"] != "Authorised use only" {
							return fmt.Errorf("CustomLoginBanner: got %v", p["CustomLoginBanner"])
						}
						ap, _ := p["AutoPatchSettings"].(map[string]interface{})
						if ap["Enabled"] != true || ap["PatchCron"] != "0 4 * * *" || len(ap) != 2 {
							return fmt.Errorf("AutoPatchSettings: expected only the configured values, got %v", ap)
						}
						edge, _ := p["Edge"].(map[string]interface{})
						mtls, _ := edge["MTLS"].(map[string]interface{})
						if edge["PingInterval"] != float64(10) || edge["TunnelServerAddress"] != "portainer.example.com:8000" || len(edge) != 3 {
							return fmt.Errorf("Edge: expected only the configured values, got %v", edge)
						}
						if mtls["UseSeparateCert"] != true || mtls["CaCert"] != "CA" || mtls["Cert"] != "CERT" || mtls["Key"] != "KEY" {
							return fmt.Errorf("Edge.MTLS: got %v", mtls)
						}
						return nil
					},
				),
			},
			{
				// Portainer keeps file paths, never the certificates.
				PreConfig: func() {
					fake.set("edge", map[string]interface{}{"PingInterval": 10, "TunnelServerAddress": "portainer.example.com:8000", "MTLS": map[string]interface{}{"UseSeparateCert": true, "KeyFile": "/data/key"}})
				},
				Config:   config,
				PlanOnly: true,
			},
			{
				PreConfig:          func() { fake.set("customloginbanner", "changed") },
				Config:             config,
				PlanOnly:           true,
				ExpectNonEmptyPlan: true,
			},
			{
				Config: mockProviderConfig(mock) + `resource "portainer_settings" "s" { logo_url = "https://example.com/logo.png" }`,
				Check: func(*terraform.State) error {
					p := gsbBody(t, mock, "PUT", "/settings")
					for _, key := range []string{"CustomLoginBanner", "AutoPatchSettings", "Edge"} {
						if _, ok := p[key]; ok {
							return fmt.Errorf("payload.%s: expected it left out when not configured", key)
						}
					}
					return nil
				},
			},
		},
	})
}

// TestSettingsResource_BusinessEditionFields_UpgradeFromV300: a Business
// Edition server that has a banner, auto-patching and Edge settings plans no
// change after upgrading, with configurations that do not set them.
func TestSettingsResource_BusinessEditionFields_UpgradeFromV300(t *testing.T) {
	t.Parallel()
	newMock := func(t *testing.T) *MockServer {
		m := NewMockServer(t)
		s := portainerDefaultSettings()
		s["customloginbanner"] = "Authorised use only"
		s["autopatchsettings"] = map[string]interface{}{"Enabled": true, "PatchCron": "0 4 * * *", "PortainerRepository": "portainer/portainer-ee"}
		s["edge"] = map[string]interface{}{"PingInterval": 5, "MTLS": map[string]interface{}{"UseSeparateCert": false}}
		newFakePortainerSettings(m, s)
		return m
	}
	gsbUpgradeTest(t, newMock,
		`resource "portainer_settings" "s" { enable_edge_compute_features = true }`,
		settingsFullConfig,
	)
}
