package internal

import (
	"net/http"
	"testing"
)

// These tests exercise the LDAP Apply/Read branches the base suite leaves
// untouched: the optional list/toggle fields (urls, admin_auto_populate,
// admin_groups), the group/admin-group search sub-blocks, the tls_config build,
// and the Read-side flatten + sensitive-value preservation branches.

// TestLDAPSettingsCov2_Apply_AllOptionalBlocks flips every remaining GetOk
// branch in resourceLDAPSettingsApply so the full payload builder runs.
func TestLDAPSettingsCov2_Apply_AllOptionalBlocks(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/settings", RespondJSON(http.StatusOK, map[string]interface{}{
		"AuthenticationMethod": 2,
		"LDAPSettings":         map[string]interface{}{"AnonymousMode": true},
	}))
	mock.On("PUT", "/settings", RespondJSON(http.StatusOK, map[string]interface{}{}))

	r := resourceLDAPSettings()
	d := r.TestResourceData()
	_ = d.Set("url", "ldap.example.com:389")
	_ = d.Set("urls", []interface{}{"ldap://a.example.com:389", "ldap://b.example.com:389"})
	_ = d.Set("anonymous_mode", true)
	_ = d.Set("auto_create_users", true)
	_ = d.Set("start_tls", true)
	_ = d.Set("server_type", 1)
	_ = d.Set("admin_auto_populate", true)
	_ = d.Set("admin_groups", []interface{}{"admins", "ops"})
	_ = d.Set("group_search_settings", []interface{}{
		map[string]interface{}{
			"group_attribute": "member",
			"group_base_dn":   "ou=groups,dc=example,dc=com",
			"group_filter":    "(objectClass=groupOfNames)",
		},
	})
	_ = d.Set("admin_group_search_settings", []interface{}{
		map[string]interface{}{
			"group_attribute": "member",
			"group_base_dn":   "ou=admin-groups,dc=example,dc=com",
			"group_filter":    "(objectClass=groupOfNames)",
		},
	})
	_ = d.Set("tls_config", []interface{}{
		map[string]interface{}{
			"tls":             true,
			"tls_ca_cert":     "ca-pem",
			"tls_cert":        "cert-pem",
			"tls_key":         "key-pem",
			"tls_skip_verify": true,
		},
	})

	if err := rcCreate(r, d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if d.Id() != "portainer-ldap-settings" {
		t.Errorf("expected ID %q, got %q", "portainer-ldap-settings", d.Id())
	}

	put := mock.FindRequest("PUT", "/settings")
	if put == nil {
		t.Fatal("expected a PUT /settings")
	}
	var payload map[string]interface{}
	if err := put.DecodeJSON(&payload); err != nil {
		t.Fatalf("failed to decode PUT body: %v", err)
	}
	ldap, ok := payload["ldapsettings"].(map[string]interface{})
	if !ok {
		t.Fatalf("ldapsettings: expected object, got %T", payload["ldapsettings"])
	}

	if urls, ok := ldap["URLs"].([]interface{}); !ok || len(urls) != 2 {
		t.Errorf("ldapsettings.URLs: expected 2 entries, got %v", ldap["URLs"])
	}
	if got := ldap["AdminAutoPopulate"]; got != true {
		t.Errorf("ldapsettings.AdminAutoPopulate: expected true, got %v", got)
	}
	if groups, ok := ldap["AdminGroups"].([]interface{}); !ok || len(groups) != 2 {
		t.Errorf("ldapsettings.AdminGroups: expected 2 entries, got %v", ldap["AdminGroups"])
	}
	if gss, ok := ldap["GroupSearchSettings"].([]interface{}); !ok || len(gss) != 1 {
		t.Errorf("ldapsettings.GroupSearchSettings: expected 1 entry, got %v", ldap["GroupSearchSettings"])
	}
	if agss, ok := ldap["AdminGroupSearchSettings"].([]interface{}); !ok || len(agss) != 1 {
		t.Errorf("ldapsettings.AdminGroupSearchSettings: expected 1 entry, got %v", ldap["AdminGroupSearchSettings"])
	}
	tls, ok := ldap["TLSConfig"].(map[string]interface{})
	if !ok {
		t.Fatalf("ldapsettings.TLSConfig: expected object, got %T", ldap["TLSConfig"])
	}
	if got := tls["TLSCACert"]; got != "ca-pem" {
		t.Errorf("TLSConfig.TLSCACert: expected %q, got %v", "ca-pem", got)
	}
	if got := tls["TLSKey"]; got != "key-pem" {
		t.Errorf("TLSConfig.TLSKey: expected %q, got %v", "key-pem", got)
	}
}

// TestLDAPSettingsCov2_Read_ExtraFieldsAndPreserve covers the Read flatten
// branches for URLs, AdminAutoPopulate, AdminGroups and AdminGroupSearchSettings,
// plus preservation of the sensitive password and tls_key from prior state.
func TestLDAPSettingsCov2_Read_ExtraFieldsAndPreserve(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/settings", RespondJSON(http.StatusOK, map[string]interface{}{
		"AuthenticationMethod": 2,
		"LDAPSettings": map[string]interface{}{
			"URL":               "ldap.example.com:389",
			"URLs":              []interface{}{"ldap://a:389", "ldap://b:389"},
			"AutoCreateUsers":   true,
			"AdminAutoPopulate": true,
			"AdminGroups":       []interface{}{"admins", "ops"},
			"ServerType":        1,
			"AdminGroupSearchSettings": []interface{}{
				map[string]interface{}{
					"GroupAttribute": "member",
					"GroupBaseDN":    "ou=admin-groups,dc=example,dc=com",
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
	// Prime sensitive values so the preservation branches run.
	_ = d.Set("password", "kept-pw")
	_ = d.Set("tls_config", []interface{}{
		map[string]interface{}{
			"tls":     true,
			"tls_key": "kept-key",
		},
	})

	if err := rcRead(r, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	if got := d.Get("admin_auto_populate"); got != true {
		t.Errorf("admin_auto_populate: expected true, got %v", got)
	}
	urls := d.Get("urls").([]interface{})
	if len(urls) != 2 {
		t.Errorf("urls: expected 2 entries, got %d", len(urls))
	}
	adminGroups := d.Get("admin_groups").([]interface{})
	if len(adminGroups) != 2 {
		t.Errorf("admin_groups: expected 2 entries, got %d", len(adminGroups))
	}
	agss := d.Get("admin_group_search_settings").([]interface{})
	if len(agss) != 1 {
		t.Errorf("admin_group_search_settings: expected 1 entry, got %d", len(agss))
	}
	if got := d.Get("password"); got != "kept-pw" {
		t.Errorf("password: expected preserved %q, got %v", "kept-pw", got)
	}
	tc := d.Get("tls_config").([]interface{})
	if len(tc) != 1 {
		t.Fatalf("tls_config: expected 1 entry, got %d", len(tc))
	}
	if got := tc[0].(map[string]interface{})["tls_key"]; got != "kept-key" {
		t.Errorf("tls_config.tls_key: expected preserved %q, got %v", "kept-key", got)
	}
}
