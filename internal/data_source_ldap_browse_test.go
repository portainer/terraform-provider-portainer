package internal

import (
	"net/http"
	"strings"
	"testing"
)

// =========================================================================
// Wave three, LDAP browsing and user authorizations. The browse endpoints
// take a full settings object rather than reusing the instance's saved
// configuration, which is what lets a configuration be tried out before it
// is committed with portainer_ldap_settings.
// =========================================================================

// resourceSetter is the part of *schema.ResourceData these helpers need.
type resourceSetter interface {
	Set(string, interface{}) error
}

func ldapConnectionArgs(d resourceSetter) {
	_ = d.Set("url", "ldap.example.com:389")
	_ = d.Set("reader_dn", "cn=readonly,dc=example,dc=com")
	_ = d.Set("password", "s3cret")
	_ = d.Set("start_tls", true)
	_ = d.Set("tls_skip_verify", true)
}

// TestDataSourceLDAPUsers_SendsSettingsAndSearches pins the payload: the whole
// directory configuration travels with the request, nested under LDAPSettings.
func TestDataSourceLDAPUsers_SendsSettingsAndSearches(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("POST", "/ldap/users", RespondJSON(http.StatusOK, []map[string]interface{}{
		{"Name": "alice", "Groups": []string{"platform", "oncall"}},
		{"Name": "bob", "Groups": []string{}},
	}))

	ds := dataSourceLDAPUsers()
	d := ds.TestResourceData()
	ldapConnectionArgs(d)
	_ = d.Set("search", []interface{}{map[string]interface{}{
		"base_dn": "ou=people,dc=example,dc=com",
		"filter":  "(objectClass=person)", "user_name_attribute": "uid",
	}})

	if err := rcRead(ds, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	var payload struct {
		LDAPSettings struct {
			URL           string `json:"URL"`
			ReaderDN      string `json:"ReaderDN"`
			Password      string `json:"Password"`
			StartTLS      bool   `json:"StartTLS"`
			AnonymousMode bool   `json:"AnonymousMode"`
			TLSConfig     struct {
				TLSSkipVerify bool `json:"TLSSkipVerify"`
			} `json:"TLSConfig"`
			SearchSettings []struct {
				BaseDN            string `json:"BaseDN"`
				Filter            string `json:"Filter"`
				UserNameAttribute string `json:"UserNameAttribute"`
			} `json:"SearchSettings"`
		} `json:"LDAPSettings"`
	}
	if err := mock.FindRequest("POST", "/ldap/users").DecodeJSON(&payload); err != nil {
		t.Fatalf("payload is not JSON: %v", err)
	}
	settings := payload.LDAPSettings
	if settings.URL != "ldap.example.com:389" || settings.ReaderDN == "" || settings.Password != "s3cret" {
		t.Errorf("the connection settings were not sent: %+v", settings)
	}
	if !settings.StartTLS || !settings.TLSConfig.TLSSkipVerify {
		t.Errorf("the TLS settings were not sent: %+v", settings)
	}
	if len(settings.SearchSettings) != 1 || settings.SearchSettings[0].UserNameAttribute != "uid" {
		t.Errorf("the search settings were not sent: %+v", settings.SearchSettings)
	}

	entries := d.Get("entries").([]interface{})
	if len(entries) != 2 {
		t.Fatalf("expected two users, got %d", len(entries))
	}
	alice := entries[0].(map[string]interface{})
	if alice["name"] != "alice" || len(alice["groups"].([]interface{})) != 2 {
		t.Errorf("the user was not read: %+v", alice)
	}
	if got := entries[1].(map[string]interface{})["groups"].([]interface{}); len(got) != 0 {
		t.Errorf("a user in no groups must read as an empty list, got %v", got)
	}
}

// TestDataSourceLDAPUsers_OmitsUnsetCredentials covers anonymous binding: the
// reader account fields must not be sent empty, which the directory would
// read as a bind attempt with a blank password.
func TestDataSourceLDAPUsers_OmitsUnsetCredentials(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("POST", "/ldap/users", RespondJSON(http.StatusOK, []interface{}{}))

	ds := dataSourceLDAPUsers()
	d := ds.TestResourceData()
	_ = d.Set("url", "ldap.example.com:389")
	_ = d.Set("anonymous_mode", true)

	if err := rcRead(ds, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	var payload struct {
		LDAPSettings map[string]interface{} `json:"LDAPSettings"`
	}
	if err := mock.FindRequest("POST", "/ldap/users").DecodeJSON(&payload); err != nil {
		t.Fatalf("payload is not JSON: %v", err)
	}
	for _, key := range []string{"ReaderDN", "Password"} {
		if _, ok := payload.LDAPSettings[key]; ok {
			t.Errorf("%s must be omitted when unset, got %v", key, payload.LDAPSettings[key])
		}
	}
	if payload.LDAPSettings["AnonymousMode"] != true {
		t.Error("anonymous mode must be sent")
	}
	if _, ok := payload.LDAPSettings["SearchSettings"]; ok {
		t.Error("no searches were configured, so none must be sent")
	}
}

// TestDataSourceLDAPGroups_UsesGroupSearchSettings pins the different settings
// key the group search travels under.
func TestDataSourceLDAPGroups_UsesGroupSearchSettings(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("POST", "/ldap/groups", RespondJSON(http.StatusOK, []map[string]interface{}{
		{"Name": "platform", "Groups": []string{}},
	}))

	ds := dataSourceLDAPGroups()
	d := ds.TestResourceData()
	ldapConnectionArgs(d)
	_ = d.Set("group_search", []interface{}{map[string]interface{}{
		"group_base_dn": "ou=groups,dc=example,dc=com",
		"group_filter":  "(objectClass=groupOfNames)", "group_attribute": "member",
	}})

	if err := rcRead(ds, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	var payload struct {
		LDAPSettings struct {
			GroupSearchSettings []struct {
				GroupBaseDN    string `json:"GroupBaseDN"`
				GroupAttribute string `json:"GroupAttribute"`
			} `json:"GroupSearchSettings"`
		} `json:"LDAPSettings"`
	}
	if err := mock.FindRequest("POST", "/ldap/groups").DecodeJSON(&payload); err != nil {
		t.Fatalf("payload is not JSON: %v", err)
	}
	searches := payload.LDAPSettings.GroupSearchSettings
	if len(searches) != 1 || searches[0].GroupAttribute != "member" {
		t.Errorf("the group search settings were not sent: %+v", searches)
	}
	if got := d.Get("entries").([]interface{}); len(got) != 1 {
		t.Errorf("the groups were not read: %v", got)
	}
}

// TestDataSourceLDAPAdminGroups_ReadsPlainNames pins the one endpoint in this
// group that answers with bare names rather than entry objects.
func TestDataSourceLDAPAdminGroups_ReadsPlainNames(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("POST", "/ldap/admin-groups", RespondJSON(http.StatusOK, []string{"platform-admins", "sre"}))

	ds := dataSourceLDAPAdminGroups()
	d := ds.TestResourceData()
	ldapConnectionArgs(d)
	_ = d.Set("admin_group_search", []interface{}{map[string]interface{}{
		"group_base_dn": "ou=groups,dc=example,dc=com",
	}})

	if err := rcRead(ds, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	var payload struct {
		LDAPSettings struct {
			AdminGroupSearchSettings []struct {
				GroupBaseDN string `json:"GroupBaseDN"`
			} `json:"AdminGroupSearchSettings"`
		} `json:"LDAPSettings"`
	}
	if err := mock.FindRequest("POST", "/ldap/admin-groups").DecodeJSON(&payload); err != nil {
		t.Fatalf("payload is not JSON: %v", err)
	}
	if len(payload.LDAPSettings.AdminGroupSearchSettings) != 1 {
		t.Errorf("the admin group search must travel under its own key: %+v", payload.LDAPSettings)
	}

	names := d.Get("group_names").([]interface{})
	if len(names) != 2 || names[0] != "platform-admins" {
		t.Errorf("the group names were not read: %v", names)
	}
}

// TestDataSourceLDAPLoginTest_ReportsRejection covers the default: a rejected
// login is usually the answer being asked for rather than an error.
func TestDataSourceLDAPLoginTest_ReportsRejection(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("POST", "/ldap/test", RespondJSON(http.StatusOK, map[string]interface{}{"valid": false}))

	ds := dataSourceLDAPLoginTest()
	d := ds.TestResourceData()
	ldapConnectionArgs(d)
	_ = d.Set("username", "alice")
	_ = d.Set("test_password", "wrong")

	if err := rcRead(ds, d, mock.Client()); err != nil {
		t.Fatalf("a rejected login must not fail the read by default: %v", err)
	}
	if got := d.Get("valid"); got != false {
		t.Errorf("valid: expected false, got %v", got)
	}

	var payload struct {
		Username string `json:"Username"`
		Password string `json:"Password"`
	}
	if err := mock.FindRequest("POST", "/ldap/test").DecodeJSON(&payload); err != nil {
		t.Fatalf("payload is not JSON: %v", err)
	}
	// The password under test is the one sent, not the reader account's.
	if payload.Username != "alice" || payload.Password != "wrong" {
		t.Errorf("the credentials under test were not sent: %+v", payload)
	}
}

// TestDataSourceLDAPLoginTest_CanFailThePlan covers the opt-in that turns a
// rejection into a hard failure.
func TestDataSourceLDAPLoginTest_CanFailThePlan(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("POST", "/ldap/test", RespondJSON(http.StatusOK, map[string]interface{}{"valid": false}))

	ds := dataSourceLDAPLoginTest()
	d := ds.TestResourceData()
	ldapConnectionArgs(d)
	_ = d.Set("username", "alice")
	_ = d.Set("test_password", "wrong")
	_ = d.Set("fail_on_error", true)

	err := rcRead(ds, d, mock.Client())
	if err == nil {
		t.Fatal("fail_on_error must turn a rejection into a failure")
	}
	if !strings.Contains(err.Error(), "fail_on_error") {
		t.Errorf("the error should point at the switch that changes this, got: %v", err)
	}
}

// TestDataSourceUserNamespaces_FlattensAndSorts covers the nested map
// Portainer answers with, and the sorting that keeps two identical plans from
// producing different output.
func TestDataSourceUserNamespaces_FlattensAndSorts(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("GET", "/users/7/namespaces", RespondJSON(http.StatusOK, map[string]interface{}{
		"2": map[string]interface{}{
			"zeta": map[string]bool{"K8sAccessNamespaceRead": true, "K8sAccessNamespaceWrite": false},
			"apps": map[string]bool{"K8sAccessNamespaceRead": true},
		},
		"1": map[string]interface{}{
			"default": map[string]bool{},
		},
	}))

	ds := dataSourceUserNamespaces()
	d := ds.TestResourceData()
	_ = d.Set("user_id", 7)

	if err := rcRead(ds, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	entries := d.Get("namespaces").([]interface{})
	if len(entries) != 3 {
		t.Fatalf("expected three namespace entries, got %d", len(entries))
	}

	// Environments sort first, then namespaces within them.
	first := entries[0].(map[string]interface{})
	if first["endpoint_id"] != "1" || first["namespace"] != "default" {
		t.Errorf("the entries are not sorted by environment then namespace: %+v", first)
	}
	if len(first["authorizations"].([]interface{})) != 0 {
		t.Errorf("a namespace with no grants must read as an empty list: %+v", first)
	}
	second := entries[1].(map[string]interface{})
	if second["endpoint_id"] != "2" || second["namespace"] != "apps" {
		t.Errorf("namespaces must be sorted within an environment: %+v", second)
	}

	// Only the authorizations that are switched on are listed.
	third := entries[2].(map[string]interface{})
	granted := third["authorizations"].([]interface{})
	if len(granted) != 1 || granted[0] != "K8sAccessNamespaceRead" {
		t.Errorf("only granted authorizations must be listed, got %v", granted)
	}

	if !strings.Contains(d.Get("raw").(string), "K8sAccessNamespaceWrite") {
		t.Error("the raw response must keep what the flattened list drops")
	}
}

// TestDataSourceCurrentUserAuthorizations_ReadsBothCalls covers the pair of
// endpoints this data source folds: the environment-wide authorizations and
// the per-namespace ones.
func TestDataSourceCurrentUserAuthorizations_ReadsBothCalls(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("GET", "/users/me/auth/5", RespondJSON(http.StatusOK, map[string]interface{}{
		"Authorizations": map[string]bool{
			"EndpointResourcesAccess": true,
			"DockerContainerDelete":   false,
			"DockerContainerList":     true,
		},
	}))
	mock.On("GET", "/users/me/auth/5/namespaces", RespondJSON(http.StatusOK, map[string]interface{}{
		"namespaceAuthorizations": map[string]interface{}{
			"apps": map[string]bool{"K8sAccessNamespaceRead": true},
		},
	}))

	ds := dataSourceCurrentUserAuthorizations()
	d := ds.TestResourceData()
	_ = d.Set("endpoint_id", 5)

	if err := rcRead(ds, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	granted := d.Get("authorizations").([]interface{})
	if len(granted) != 2 {
		t.Fatalf("only granted authorizations must be listed, got %v", granted)
	}
	// Sorted, so the output is stable between two identical plans.
	if granted[0] != "DockerContainerList" || granted[1] != "EndpointResourcesAccess" {
		t.Errorf("the authorizations must be sorted, got %v", granted)
	}

	namespaces := d.Get("namespace_authorizations").([]interface{})
	if len(namespaces) != 1 {
		t.Fatalf("expected one namespace, got %d", len(namespaces))
	}
	entry := namespaces[0].(map[string]interface{})
	if entry["namespace"] != "apps" {
		t.Errorf("the namespace was not read: %+v", entry)
	}
}

// TestDataSourceCurrentUserAuthorizations_EmptyForAdmins covers the documented
// case where the per-namespace map comes back empty.
func TestDataSourceCurrentUserAuthorizations_EmptyForAdmins(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("GET", "/users/me/auth/5", RespondJSON(http.StatusOK, map[string]interface{}{
		"Authorizations": map[string]bool{},
	}))
	mock.On("GET", "/users/me/auth/5/namespaces", RespondJSON(http.StatusOK, map[string]interface{}{}))

	ds := dataSourceCurrentUserAuthorizations()
	d := ds.TestResourceData()
	_ = d.Set("endpoint_id", 5)

	if err := rcRead(ds, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if got := d.Get("namespace_authorizations").([]interface{}); len(got) != 0 {
		t.Errorf("an empty map must read as an empty list, got %v", got)
	}
}
