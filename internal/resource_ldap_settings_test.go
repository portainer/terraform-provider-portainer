package internal

import (
	"net/http"
	"testing"
)

// resource_ldap_settings performs a read-modify-write on the shared /settings
// object: Create/Update first GET /settings to fetch the full current payload,
// then PUT /settings with authenticationMethod=2 and the merged ldapsettings
// section. The mock harness records both calls so we can assert the PUT body.

// TestLDAPSettingsCreate_HappyPath verifies the GET/PUT sequence and that the
// PUT payload carries the configured LDAP fields. The provider sends only the
// authenticationMethod + ldapsettings keys (it does NOT echo back the unrelated
// fields from GET), so we assert the LDAP section is well-formed.
func TestLDAPSettingsCreate_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	// Current settings returned by the initial GET. Includes non-LDAP fields
	// that the read-modify-write reads in. The PUT response is what the
	// chained Read consumes.
	mock.On("GET", "/settings", RespondJSON(http.StatusOK, map[string]interface{}{
		"AuthenticationMethod": 1,
		"SnapshotInterval":     "5m",
		"LDAPSettings": map[string]interface{}{
			"URL":        "ldap.example.com:389",
			"ReaderDN":   "cn=readonly,dc=example,dc=com",
			"ServerType": 0,
			"SearchSettings": []map[string]interface{}{
				{
					"BaseDN":            "dc=example,dc=com",
					"Filter":            "(objectClass=person)",
					"UserNameAttribute": "uid",
				},
			},
		},
	}))
	mock.On("PUT", "/settings", RespondJSON(http.StatusOK, map[string]interface{}{}))

	r := resourceLDAPSettings()
	d := r.TestResourceData()
	_ = d.Set("url", "ldap.example.com:389")
	_ = d.Set("reader_dn", "cn=readonly,dc=example,dc=com")
	_ = d.Set("password", "s3cret")
	_ = d.Set("anonymous_mode", false)
	_ = d.Set("auto_create_users", true)
	_ = d.Set("server_type", 0)
	_ = d.Set("search_settings", []interface{}{
		map[string]interface{}{
			"base_dn":             "dc=example,dc=com",
			"filter":              "(objectClass=person)",
			"user_name_attribute": "uid",
		},
	})

	if err := rcCreate(r, d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	if d.Id() != "portainer-ldap-settings" {
		t.Errorf("expected ID %q, got %q", "portainer-ldap-settings", d.Id())
	}

	// The provider must read current settings before writing.
	if mock.FindRequest("GET", "/settings") == nil {
		t.Error("expected a GET /settings before the PUT (read-modify-write)")
	}

	put := mock.FindRequest("PUT", "/settings")
	if put == nil {
		t.Fatal("expected a PUT /settings")
	}
	var payload map[string]interface{}
	if err := put.DecodeJSON(&payload); err != nil {
		t.Fatalf("failed to decode PUT body: %v", err)
	}

	// authenticationMethod is forced to 2 (LDAP). JSON numbers decode as float64.
	if got := payload["authenticationMethod"]; got != float64(2) {
		t.Errorf("authenticationMethod: expected 2, got %v", got)
	}

	ldapRaw, ok := payload["ldapsettings"]
	if !ok {
		t.Fatal("PUT payload missing ldapsettings section")
	}
	ldap, ok := ldapRaw.(map[string]interface{})
	if !ok {
		t.Fatalf("ldapsettings: expected object, got %T", ldapRaw)
	}

	if got := ldap["URL"]; got != "ldap.example.com:389" {
		t.Errorf("ldapsettings.URL: expected %q, got %v", "ldap.example.com:389", got)
	}
	if got := ldap["ReaderDN"]; got != "cn=readonly,dc=example,dc=com" {
		t.Errorf("ldapsettings.ReaderDN: expected %q, got %v", "cn=readonly,dc=example,dc=com", got)
	}
	if got := ldap["Password"]; got != "s3cret" {
		t.Errorf("ldapsettings.Password: expected %q, got %v", "s3cret", got)
	}
	if got := ldap["AutoCreateUsers"]; got != true {
		t.Errorf("ldapsettings.AutoCreateUsers: expected true, got %v", got)
	}

	// SearchSettings must be carried through with the camelCase API keys.
	ssRaw, ok := ldap["SearchSettings"].([]interface{})
	if !ok || len(ssRaw) != 1 {
		t.Fatalf("ldapsettings.SearchSettings: expected 1 entry, got %v", ldap["SearchSettings"])
	}
	ss := ssRaw[0].(map[string]interface{})
	if got := ss["BaseDN"]; got != "dc=example,dc=com" {
		t.Errorf("SearchSettings.BaseDN: expected %q, got %v", "dc=example,dc=com", got)
	}
	if got := ss["Filter"]; got != "(objectClass=person)" {
		t.Errorf("SearchSettings.Filter: expected %q, got %v", "(objectClass=person)", got)
	}
	if got := ss["UserNameAttribute"]; got != "uid" {
		t.Errorf("SearchSettings.UserNameAttribute: expected %q, got %v", "uid", got)
	}
}

// TestLDAPSettingsRead_HappyPath verifies that GET /settings maps the
// LDAPSettings sub-object into the resource state.
func TestLDAPSettingsRead_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/settings", RespondJSON(http.StatusOK, map[string]interface{}{
		"AuthenticationMethod": 2,
		"LDAPSettings": map[string]interface{}{
			"AnonymousMode":   false,
			"AutoCreateUsers": true,
			"ReaderDN":        "cn=readonly,dc=example,dc=com",
			"StartTLS":        true,
			"URL":             "ldap.example.com:389",
			"ServerType":      1,
			"SearchSettings": []interface{}{
				map[string]interface{}{
					"BaseDN":            "dc=example,dc=com",
					"Filter":            "(objectClass=person)",
					"UserNameAttribute": "uid",
				},
			},
			"GroupSearchSettings": []interface{}{
				map[string]interface{}{
					"GroupAttribute": "member",
					"GroupBaseDN":    "ou=groups,dc=example,dc=com",
					"GroupFilter":    "(objectClass=groupOfNames)",
				},
			},
			"TLSConfig": map[string]interface{}{
				"TLS":           true,
				"TLSCACert":     "ca-cert-data",
				"TLSCert":       "cert-data",
				"TLSSkipVerify": false,
			},
		},
	}))

	r := resourceLDAPSettings()
	d := r.TestResourceData()
	d.SetId("portainer-ldap-settings")

	if err := rcRead(r, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	if d.Id() != "portainer-ldap-settings" {
		t.Errorf("expected ID retained, got %q", d.Id())
	}
	if got := d.Get("reader_dn"); got != "cn=readonly,dc=example,dc=com" {
		t.Errorf("reader_dn: expected %q, got %v", "cn=readonly,dc=example,dc=com", got)
	}
	if got := d.Get("url"); got != "ldap.example.com:389" {
		t.Errorf("url: expected %q, got %v", "ldap.example.com:389", got)
	}
	if got := d.Get("auto_create_users"); got != true {
		t.Errorf("auto_create_users: expected true, got %v", got)
	}
	if got := d.Get("start_tls"); got != true {
		t.Errorf("start_tls: expected true, got %v", got)
	}
	// ServerType arrives as JSON float64 and is converted to int.
	if got := d.Get("server_type"); got != 1 {
		t.Errorf("server_type: expected 1, got %v", got)
	}

	ss := d.Get("search_settings").([]interface{})
	if len(ss) != 1 {
		t.Fatalf("search_settings: expected 1 entry, got %d", len(ss))
	}
	s := ss[0].(map[string]interface{})
	if got := s["base_dn"]; got != "dc=example,dc=com" {
		t.Errorf("search_settings.base_dn: expected %q, got %v", "dc=example,dc=com", got)
	}
	if got := s["user_name_attribute"]; got != "uid" {
		t.Errorf("search_settings.user_name_attribute: expected %q, got %v", "uid", got)
	}

	gss := d.Get("group_search_settings").([]interface{})
	if len(gss) != 1 {
		t.Fatalf("group_search_settings: expected 1 entry, got %d", len(gss))
	}
	g := gss[0].(map[string]interface{})
	if got := g["group_base_dn"]; got != "ou=groups,dc=example,dc=com" {
		t.Errorf("group_search_settings.group_base_dn: expected %q, got %v", "ou=groups,dc=example,dc=com", got)
	}

	tc := d.Get("tls_config").([]interface{})
	if len(tc) != 1 {
		t.Fatalf("tls_config: expected 1 entry, got %d", len(tc))
	}
	tlsm := tc[0].(map[string]interface{})
	if got := tlsm["tls"]; got != true {
		t.Errorf("tls_config.tls: expected true, got %v", got)
	}
	if got := tlsm["tls_ca_cert"]; got != "ca-cert-data" {
		t.Errorf("tls_config.tls_ca_cert: expected %q, got %v", "ca-cert-data", got)
	}
}

// TestLDAPSettingsRead_NoLDAPSection verifies that when /settings has no
// LDAPSettings sub-object, the resource clears its ID (drift detection).
func TestLDAPSettingsRead_NoLDAPSection(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/settings", RespondJSON(http.StatusOK, map[string]interface{}{
		"AuthenticationMethod": 1,
	}))

	r := resourceLDAPSettings()
	d := r.TestResourceData()
	d.SetId("portainer-ldap-settings")

	if err := rcRead(r, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if d.Id() != "" {
		t.Errorf("expected ID cleared when LDAPSettings absent, got %q", d.Id())
	}
}

// TestLDAPSettingsUpdate_HappyPath verifies Update reuses the same apply path:
// it reads current settings then PUTs the new LDAP configuration.
func TestLDAPSettingsUpdate_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/settings", RespondJSON(http.StatusOK, map[string]interface{}{
		"AuthenticationMethod": 2,
		"LDAPSettings": map[string]interface{}{
			"URL":        "old.example.com:389",
			"ServerType": 0,
		},
	}))
	mock.On("PUT", "/settings", RespondJSON(http.StatusOK, map[string]interface{}{}))

	r := resourceLDAPSettings()
	d := r.TestResourceData()
	d.SetId("portainer-ldap-settings")
	_ = d.Set("url", "new.example.com:636")
	_ = d.Set("start_tls", true)
	_ = d.Set("server_type", 0)

	if err := rcUpdate(r, d, mock.Client()); err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	put := mock.FindRequest("PUT", "/settings")
	if put == nil {
		t.Fatal("expected a PUT /settings on update")
	}
	var payload map[string]interface{}
	if err := put.DecodeJSON(&payload); err != nil {
		t.Fatalf("failed to decode PUT body: %v", err)
	}
	ldap := payload["ldapsettings"].(map[string]interface{})
	if got := ldap["URL"]; got != "new.example.com:636" {
		t.Errorf("ldapsettings.URL: expected updated %q, got %v", "new.example.com:636", got)
	}
	if got := ldap["StartTLS"]; got != true {
		t.Errorf("ldapsettings.StartTLS: expected true, got %v", got)
	}
}

// TestLDAPSettingsDelete verifies Delete is a no-op that just clears state
// (LDAP settings live on the shared /settings object and are not "deleted").
func TestLDAPSettingsDelete(t *testing.T) {
	mock := NewMockServer(t)

	r := resourceLDAPSettings()
	d := r.TestResourceData()
	d.SetId("portainer-ldap-settings")

	if err := rcDelete(r, d, mock.Client()); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	if d.Id() != "" {
		t.Errorf("expected ID cleared after Delete, got %q", d.Id())
	}
	// Delete must not touch the API.
	if len(mock.Requests()) != 0 {
		t.Errorf("expected no API requests on Delete, got %d", len(mock.Requests()))
	}
}

// TestLDAPSettingsCreate_GETError verifies an HTTP error on the initial
// GET /settings is propagated and the ID is not set.
func TestLDAPSettingsCreate_GETError(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/settings", RespondString(
		http.StatusInternalServerError, "application/json",
		`{"message":"boom"}`,
	))

	r := resourceLDAPSettings()
	d := r.TestResourceData()
	_ = d.Set("url", "ldap.example.com:389")

	err := rcCreate(r, d, mock.Client())
	if err == nil {
		t.Fatal("expected error when GET /settings fails, got nil")
	}
	if d.Id() != "" {
		t.Errorf("expected empty ID after GET error, got %q", d.Id())
	}
}

// TestLDAPSettingsCreate_PUTError verifies an HTTP error on the PUT /settings
// (after a successful GET) is propagated and the ID is not set.
func TestLDAPSettingsCreate_PUTError(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/settings", RespondJSON(http.StatusOK, map[string]interface{}{
		"AuthenticationMethod": 1,
		"LDAPSettings":         map[string]interface{}{},
	}))
	mock.On("PUT", "/settings", RespondString(
		http.StatusBadRequest, "application/json",
		`{"message":"invalid ldap config"}`,
	))

	r := resourceLDAPSettings()
	d := r.TestResourceData()
	_ = d.Set("url", "ldap.example.com:389")

	err := rcCreate(r, d, mock.Client())
	if err == nil {
		t.Fatal("expected error when PUT /settings fails, got nil")
	}
	if d.Id() != "" {
		t.Errorf("expected empty ID after PUT error, got %q", d.Id())
	}
}
