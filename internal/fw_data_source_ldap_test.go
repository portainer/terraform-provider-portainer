package internal

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

const ldapConnectionHCL = `
  url             = "ldap.example.com:389"
  reader_dn       = "cn=reader,dc=example,dc=com"
  password        = "s3cret"
  start_tls       = true
  tls_skip_verify = true`

// ldapPayload decodes the LDAPSettings object of the last request to path.
func ldapPayload(t *testing.T, mock *MockServer, path string) map[string]interface{} {
	t.Helper()
	settings, ok := gsbBody(t, mock, "POST", path)["LDAPSettings"].(map[string]interface{})
	if !ok {
		t.Fatalf("payload should nest the settings under LDAPSettings")
	}
	return settings
}

// TestLDAPUsersDataSource_SendsSettingsAndSearches pins the payload: the whole
// directory configuration travels with the request, nested under LDAPSettings.
func TestLDAPUsersDataSource_SendsSettingsAndSearches(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("POST", "/ldap/users", RespondJSON(http.StatusOK, []map[string]interface{}{
		{"Name": "alice", "Groups": []string{"platform", "oncall"}},
		{"Name": "bob", "Groups": []string{}},
	}))
	const ds = "data.portainer_ldap_users.u"
	dataSourceTest(t, mock, fmt.Sprintf(`
data "portainer_ldap_users" "u" {
  %s
  search {
    base_dn             = "ou=people,dc=example,dc=com"
    filter              = "(objectClass=person)"
    user_name_attribute = "uid"
  }
}`, ldapConnectionHCL), resource.ComposeTestCheckFunc(
		resource.TestCheckResourceAttr(ds, "id", "portainer-ldap-users-ldap.example.com:389"),
		resource.TestCheckResourceAttr(ds, "entries.#", "2"),
		resource.TestCheckResourceAttr(ds, "entries.0.name", "alice"),
		resource.TestCheckResourceAttr(ds, "entries.0.groups.#", "2"),
		resource.TestCheckResourceAttr(ds, "entries.1.groups.#", "0"),
		func(*terraform.State) error {
			s := ldapPayload(t, mock, "/ldap/users")
			if s["URL"] != "ldap.example.com:389" || s["ReaderDN"] == nil || s["Password"] != "s3cret" {
				return fmt.Errorf("the connection settings were not sent: %v", s)
			}
			if s["StartTLS"] != true || s["TLSConfig"].(map[string]interface{})["TLSSkipVerify"] != true {
				return fmt.Errorf("the TLS settings were not sent: %v", s)
			}
			search, _ := s["SearchSettings"].([]interface{})
			if len(search) != 1 || search[0].(map[string]interface{})["UserNameAttribute"] != "uid" {
				return fmt.Errorf("the search settings were not sent: %v", s["SearchSettings"])
			}
			return nil
		},
	))
}

// TestLDAPUsersDataSource_OmitsUnsetCredentials covers anonymous binding: the
// reader account fields must not be sent empty, which the directory would
// read as a bind attempt with a blank password.
func TestLDAPUsersDataSource_OmitsUnsetCredentials(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("POST", "/ldap/users", RespondJSON(http.StatusOK, []interface{}{}))
	dataSourceTest(t, mock, `
data "portainer_ldap_users" "u" {
  url            = "ldap.example.com:389"
  anonymous_mode = true
}`, resource.ComposeTestCheckFunc(
		resource.TestCheckResourceAttr("data.portainer_ldap_users.u", "start_tls", "false"),
		func(*terraform.State) error {
			s := ldapPayload(t, mock, "/ldap/users")
			for _, key := range []string{"ReaderDN", "Password", "SearchSettings"} {
				if _, ok := s[key]; ok {
					return fmt.Errorf("%s must be omitted when unset, got %v", key, s[key])
				}
			}
			if s["AnonymousMode"] != true || s["StartTLS"] != false {
				return fmt.Errorf("anonymous mode must be sent: %v", s)
			}
			return nil
		},
	))
}

// TestLDAPGroupsDataSource_UsesGroupSearchSettings pins the settings key the
// group search travels under.
func TestLDAPGroupsDataSource_UsesGroupSearchSettings(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("POST", "/ldap/groups", RespondJSON(http.StatusOK, []map[string]interface{}{
		{"Name": "cn=platform,ou=groups,dc=example,dc=com", "Groups": []string{"alice"}},
	}))
	dataSourceTest(t, mock, fmt.Sprintf(`
data "portainer_ldap_groups" "g" {
  %s
  group_search {
    group_base_dn = "ou=groups,dc=example,dc=com"
    group_filter  = "(objectClass=groupOfNames)"
  }
}`, ldapConnectionHCL), resource.ComposeTestCheckFunc(
		resource.TestCheckResourceAttr("data.portainer_ldap_groups.g", "entries.0.groups.0", "alice"),
		resource.TestCheckResourceAttr("data.portainer_ldap_groups.g", "id", "portainer-ldap-groups-ldap.example.com:389"),
		func(*terraform.State) error {
			search, _ := ldapPayload(t, mock, "/ldap/groups")["GroupSearchSettings"].([]interface{})
			if len(search) != 1 {
				return fmt.Errorf("expected one group search, got %v", search)
			}
			entry := search[0].(map[string]interface{})
			if entry["GroupBaseDN"] != "ou=groups,dc=example,dc=com" || entry["GroupFilter"] != "(objectClass=groupOfNames)" {
				return fmt.Errorf("group search mismatch: %v", entry)
			}
			if _, ok := entry["GroupAttribute"]; ok {
				return fmt.Errorf("an unset group attribute must be omitted")
			}
			return nil
		},
	))
}

// TestLDAPAdminGroupsDataSource_ReadsPlainNames: this endpoint answers with a
// plain list of names.
func TestLDAPAdminGroupsDataSource_ReadsPlainNames(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("POST", "/ldap/admin-groups", RespondJSON(http.StatusOK, []string{"admins", "ops"}))
	dataSourceTest(t, mock, fmt.Sprintf(`
data "portainer_ldap_admin_groups" "a" {
  %s
  admin_group_search {
    group_base_dn = "ou=groups,dc=example,dc=com"
  }
}`, ldapConnectionHCL), resource.ComposeTestCheckFunc(
		resource.TestCheckResourceAttr("data.portainer_ldap_admin_groups.a", "group_names.#", "2"),
		resource.TestCheckResourceAttr("data.portainer_ldap_admin_groups.a", "group_names.1", "ops"),
		func(*terraform.State) error {
			if _, ok := ldapPayload(t, mock, "/ldap/admin-groups")["AdminGroupSearchSettings"]; !ok {
				return fmt.Errorf("the admin group search must travel under AdminGroupSearchSettings")
			}
			return nil
		},
	))
}

// TestLDAPLoginTestDataSource: a rejected login is reported by default, and
// fail_on_error turns it into an error. The tested password is not the
// reader's.
func TestLDAPLoginTestDataSource(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("POST", "/ldap/test", RespondJSON(http.StatusOK, map[string]interface{}{"valid": false}))
	config := func(extra string) string {
		return fmt.Sprintf(`
data "portainer_ldap_login_test" "l" {
  %s
  username      = "alice"
  test_password = "wrong"
  %s
  search {
    base_dn = "ou=people,dc=example,dc=com"
  }
}`, ldapConnectionHCL, extra)
	}
	dataSourceTest(t, mock, config(""), resource.ComposeTestCheckFunc(
		resource.TestCheckResourceAttr("data.portainer_ldap_login_test.l", "valid", "false"),
		resource.TestCheckResourceAttr("data.portainer_ldap_login_test.l", "fail_on_error", "false"),
		resource.TestCheckResourceAttr("data.portainer_ldap_login_test.l", "id", "portainer-ldap-login-test-alice"),
		func(*terraform.State) error {
			body := gsbBody(t, mock, "POST", "/ldap/test")
			if body["Username"] != "alice" || body["Password"] != "wrong" {
				return fmt.Errorf("payload mismatch: %v", body)
			}
			if body["LDAPSettings"].(map[string]interface{})["Password"] != "s3cret" {
				return fmt.Errorf("the reader password belongs in LDAPSettings")
			}
			return nil
		},
	))
	dataSourceErrorTest(t, mock, config("fail_on_error = true"), `the directory rejected the credentials of alice`)
}

// TestLDAPCheckDataSource: 204 reads as success; a refused bind fails the
// read by default - the point of a pre-flight check - and is reported with
// fail_on_error = false.
func TestLDAPCheckDataSource(t *testing.T) {
	t.Parallel()
	ok := NewMockServer(t)
	ok.On("POST", "/ldap/check", RespondString(http.StatusNoContent, "", ""))
	dataSourceTest(t, ok, fmt.Sprintf(`data "portainer_ldap_check" "c" {%s
}`, ldapConnectionHCL), resource.ComposeTestCheckFunc(
		resource.TestCheckResourceAttr("data.portainer_ldap_check.c", "success", "true"),
		resource.TestCheckResourceAttr("data.portainer_ldap_check.c", "error", ""),
		resource.TestCheckResourceAttr("data.portainer_ldap_check.c", "fail_on_error", "true"),
		resource.TestCheckResourceAttr("data.portainer_ldap_check.c", "id", "ldap-check-ldap.example.com:389"),
		func(*terraform.State) error {
			s := ldapPayload(t, ok, "/ldap/check")
			if s["URL"] != "ldap.example.com:389" || s["StartTLS"] != true {
				return fmt.Errorf("settings mismatch: %v", s)
			}
			if _, isMap := s["TLSConfig"].(map[string]interface{}); !isMap {
				return fmt.Errorf("TLSConfig should be nested, got %v", s["TLSConfig"])
			}
			return nil
		},
	))

	refused := NewMockServer(t)
	refused.On("POST", "/ldap/check", RespondString(http.StatusBadRequest, "application/json", `{"message":"invalid credentials"}`))
	dataSourceErrorTest(t, refused, `data "portainer_ldap_check" "c" { url = "ldap.example.com:389" }`, `invalid credentials`)
	dataSourceTest(t, refused, `
data "portainer_ldap_check" "c" {
  url           = "ldap.example.com:389"
  fail_on_error = false
}`, resource.ComposeTestCheckFunc(
		resource.TestCheckResourceAttr("data.portainer_ldap_check.c", "success", "false"),
		resource.TestMatchResourceAttr("data.portainer_ldap_check.c", "error", regexpMust(`invalid credentials`)),
	))
}
