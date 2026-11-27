package internal

import (
	"fmt"
	"net/http"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

const ldapSettingsConfig = `
resource "portainer_ldap_settings" "l" {
  reader_dn         = "cn=reader,dc=example,dc=com"
  password          = "ldap-secret"
  url               = "ldap.example.com:389"
  urls              = ["ldap.example.com:389"]
  server_type       = 1
  auto_create_users = true
  admin_auto_populate = true
  admin_groups      = ["admins"]

  search_settings {
    base_dn             = "ou=people,dc=example,dc=com"
    filter              = "(objectClass=person)"
    user_name_attribute = "uid"
  }
  group_search_settings {
    group_base_dn   = "ou=groups,dc=example,dc=com"
    group_attribute = "member"
  }
  admin_group_search_settings {
    group_base_dn = "ou=admins,dc=example,dc=com"
  }
  tls_config {
    tls         = true
    tls_ca_cert = "CA"
    tls_key     = "KEY"
  }
}`

// TestLDAPSettingsResource_Lifecycle: GET then PUT with the LDAP method (2)
// and every field, secrets kept in state, an empty plan, drift and import.
func TestLDAPSettingsResource_Lifecycle(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	fake := newFakePortainerSettings(mock, portainerDefaultSettings())
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{
				Config: mockProviderConfig(mock) + ldapSettingsConfig,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("portainer_ldap_settings.l", "id", "portainer-ldap-settings"),
					resource.TestCheckResourceAttr("portainer_ldap_settings.l", "anonymous_mode", "false"),
					resource.TestCheckResourceAttr("portainer_ldap_settings.l", "password", "ldap-secret"),
					resource.TestCheckResourceAttr("portainer_ldap_settings.l", "tls_config.0.tls_key", "KEY"),
					func(*terraform.State) error {
						p := gsbBody(t, mock, "PUT", "/settings")
						if p["authenticationMethod"] != float64(2) {
							return fmt.Errorf("authenticationMethod: got %v", p["authenticationMethod"])
						}
						l := p["ldapsettings"].(map[string]interface{})
						for k, want := range map[string]interface{}{
							"ReaderDN": "cn=reader,dc=example,dc=com", "Password": "ldap-secret", "URL": "ldap.example.com:389",
							"ServerType": float64(1), "AutoCreateUsers": true, "AdminAutoPopulate": true,
						} {
							if l[k] != want {
								return fmt.Errorf("ldapsettings.%s: got %v, want %v", k, l[k], want)
							}
						}
						if _, ok := l["AnonymousMode"]; ok {
							return fmt.Errorf("a false anonymous_mode is left out, as before")
						}
						if len(l["AdminGroupSearchSettings"].([]interface{})) != 1 || l["TLSConfig"].(map[string]interface{})["TLSKey"] != "KEY" {
							return fmt.Errorf("ldapsettings blocks mismatch: %v", l)
						}
						return nil
					},
				),
			},
			{
				PreConfig: func() {
					fake.mu.Lock()
					fake.settings["ldapsettings"].(map[string]interface{})["ReaderDN"] = "cn=other"
					fake.mu.Unlock()
				},
				Config:             mockProviderConfig(mock) + ldapSettingsConfig,
				PlanOnly:           true,
				ExpectNonEmptyPlan: true,
			},
			{Config: mockProviderConfig(mock) + ldapSettingsConfig},
			{
				ResourceName:            "portainer_ldap_settings.l",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"password", "tls_config.0.tls_key"},
			},
		},
	})
}

// TestLDAPSettingsResource_NoLDAPSection: without LDAP settings in Portainer
// the resource is gone.
func TestLDAPSettingsResource_NoLDAPSection(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	fake := newFakePortainerSettings(mock, nil)
	config := mockProviderConfig(mock) + `resource "portainer_ldap_settings" "l" { url = "ldap.example.com:389" }`
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{Config: config},
			{
				PreConfig: func() {
					fake.mu.Lock()
					delete(fake.settings, "ldapsettings")
					fake.mu.Unlock()
				},
				Config:             config,
				PlanOnly:           true,
				ExpectNonEmptyPlan: true,
			},
		},
	})
}

func TestLDAPSettingsResource_Errors(t *testing.T) {
	t.Parallel()
	get := NewMockServer(t)
	get.On("GET", "/settings", RespondString(http.StatusInternalServerError, "application/json", `{"message":"boom"}`))
	put := NewMockServer(t)
	put.On("GET", "/settings", RespondJSON(http.StatusOK, map[string]interface{}{}))
	put.On("PUT", "/settings", RespondString(http.StatusBadRequest, "application/json", `{"message":"bad"}`))
	config := `resource "portainer_ldap_settings" "l" { url = "ldap.example.com:389" }`
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{Config: mockProviderConfig(get) + config, ExpectError: regexp.MustCompile(`failed to read current settings`)},
			{Config: mockProviderConfig(put) + config, ExpectError: regexp.MustCompile(`failed to update LDAP settings`)},
		},
	})
}

// TestLDAPSettingsResource_WriteOnlySecrets: moving to password_wo and
// tls_key_wo sends the write-only values and clears the old plaintext ones
// from state.
func TestLDAPSettingsResource_WriteOnlySecrets(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	newFakePortainerSettings(mock, nil)
	plain := mockProviderConfig(mock) + `
resource "portainer_ldap_settings" "l" {
  reader_dn = "cn=reader"
  url       = "ldap.example.com:636"
  password  = "old-plaintext"
  tls_config {
    tls     = true
    tls_key = "old-key"
  }
}`
	wo := mockProviderConfig(mock) + `
resource "portainer_ldap_settings" "l" {
  reader_dn           = "cn=reader"
  url                 = "ldap.example.com:636"
  password_wo         = "ldap-secret-2"
  password_wo_version = 1
  tls_key_wo          = "new-key"
  tls_key_wo_version  = 1
  tls_config {
    tls = true
  }
}`
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{Config: plain},
			{
				Config: wo,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("portainer_ldap_settings.l", "password", ""),
					resource.TestCheckResourceAttr("portainer_ldap_settings.l", "tls_config.0.tls_key", ""),
					func(*terraform.State) error {
						l := gsbBody(t, mock, "PUT", "/settings")["ldapsettings"].(map[string]interface{})
						if l["Password"] != "ldap-secret-2" {
							return fmt.Errorf("Password: expected the write-only value, got %v", l["Password"])
						}
						if l["TLSConfig"].(map[string]interface{})["TLSKey"] != "new-key" {
							return fmt.Errorf("TLSKey: expected the write-only key")
						}
						return nil
					},
				),
			},
			{Config: wo, PlanOnly: true},
		},
	})
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{{
			Config:      mockProviderConfig(mock) + `resource "portainer_ldap_settings" "l" {` + "\n" + `tls_key_wo = "k"` + "\n" + `tls_key_wo_version = 1` + "\n}",
			ExpectError: regexp.MustCompile(`tls_config,tls_key_wo,tls_key_wo_version`),
		}},
	})
}

func TestLDAPSettingsResource_UpgradeFromV300(t *testing.T) {
	t.Parallel()
	newMock := func(t *testing.T) *MockServer {
		m := NewMockServer(t)
		newFakePortainerSettings(m, portainerDefaultSettings())
		return m
	}
	gsbUpgradeTest(t, newMock,
		`
resource "portainer_ldap_settings" "l" {
  reader_dn = "cn=reader,dc=example,dc=com"
  password  = "p"
  url       = "ldap.example.com:389"
  search_settings {
    base_dn             = "ou=people,dc=example,dc=com"
    user_name_attribute = "uid"
  }
}`,
		ldapSettingsConfig,
		`
resource "portainer_ldap_settings" "l" {
  anonymous_mode      = true
  url                 = "ldap.example.com:389"
  password_wo         = "p"
  password_wo_version = 1
}`,
	)
}
