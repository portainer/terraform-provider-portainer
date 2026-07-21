package internal

import (
	"net/http"
	"testing"
)

// TestSettingsCov3_Read_OAuthBranch covers the OAuth flatten branches of
// resourceSettingsRead that the base happy path does not reach: the
// TeamMemberships block (with claim mappings) and preservation of the
// client_secret from prior state (never returned by the API).
func TestSettingsCov3_Read_OAuthBranch(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/settings", RespondJSON(http.StatusOK, map[string]interface{}{
		"authenticationMethod": 3,
		"oauthSettings": map[string]interface{}{
			"ClientID":                    "client-x",
			"AuthStyle":                   1,
			"SSO":                         true,
			"OAuthAutoMapTeamMemberships": true,
			"TeamMemberships": map[string]interface{}{
				"OAuthClaimName":            "groups",
				"AdminAutoPopulate":         true,
				"AdminGroupClaimsRegexList": []string{"^admin$"},
				"OAuthClaimMappings": []map[string]interface{}{
					{"ClaimValRegex": "^dev$", "Team": 2},
				},
			},
		},
	}))

	r := resourceSettings()
	d := r.TestResourceData()
	d.SetId("portainer-settings")
	// Prime state with a client_secret so the preservation branch runs.
	_ = d.Set("oauth_settings", []interface{}{
		map[string]interface{}{
			"client_id":     "client-x",
			"client_secret": "kept-secret",
		},
	})

	if err := rcRead(r, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	oauthList := d.Get("oauth_settings").([]interface{})
	if len(oauthList) != 1 {
		t.Fatalf("oauth_settings: expected 1 block, got %d", len(oauthList))
	}
	oauth := oauthList[0].(map[string]interface{})
	if oauth["client_secret"] != "kept-secret" {
		t.Errorf("client_secret: expected preserved value, got %v", oauth["client_secret"])
	}
	tmList := oauth["team_memberships"].([]interface{})
	if len(tmList) != 1 {
		t.Fatalf("team_memberships: expected 1 block, got %d", len(tmList))
	}
	tm := tmList[0].(map[string]interface{})
	if tm["oauth_claim_name"] != "groups" {
		t.Errorf("oauth_claim_name: got %v", tm["oauth_claim_name"])
	}
	mappings := tm["oauth_claim_mappings"].([]interface{})
	if len(mappings) != 1 {
		t.Fatalf("oauth_claim_mappings: expected 1, got %d", len(mappings))
	}
}

// TestSettingsCov3_Read_LDAPBranch covers the LDAP flatten branches:
// search_settings, group_search_settings, tls_config, and preservation of the
// reader password from prior state.
func TestSettingsCov3_Read_LDAPBranch(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/settings", RespondJSON(http.StatusOK, map[string]interface{}{
		"authenticationMethod": 2,
		"ldapsettings": map[string]interface{}{
			"URL":             "ldap://ldap.example.com",
			"AutoCreateUsers": true,
			"ReaderDN":        "cn=reader",
			"SearchSettings": []map[string]interface{}{
				{"BaseDN": "ou=users", "Filter": "(uid=%s)", "UserNameAttribute": "uid"},
			},
			"GroupSearchSettings": []map[string]interface{}{
				{"GroupAttribute": "member", "GroupBaseDN": "ou=groups", "GroupFilter": "(cn=*)"},
			},
			"TLSConfig": map[string]interface{}{
				"TLS":           true,
				"TLSSkipVerify": true,
				"TLSCACert":     "ca-pem",
			},
		},
	}))

	r := resourceSettings()
	d := r.TestResourceData()
	d.SetId("portainer-settings")
	// Prime state with an LDAP password so the preservation branch runs.
	_ = d.Set("ldap_settings", []interface{}{
		map[string]interface{}{
			"url":      "ldap://ldap.example.com",
			"password": "kept-pw",
		},
	})

	if err := rcRead(r, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	ldapList := d.Get("ldap_settings").([]interface{})
	if len(ldapList) != 1 {
		t.Fatalf("ldap_settings: expected 1 block, got %d", len(ldapList))
	}
	ldap := ldapList[0].(map[string]interface{})
	if ldap["password"] != "kept-pw" {
		t.Errorf("password: expected preserved value, got %v", ldap["password"])
	}
	if search := ldap["search_settings"].([]interface{}); len(search) != 1 {
		t.Errorf("search_settings: expected 1, got %d", len(search))
	}
	if grp := ldap["group_search_settings"].([]interface{}); len(grp) != 1 {
		t.Errorf("group_search_settings: expected 1, got %d", len(grp))
	}
	if tls := ldap["tls_config"].([]interface{}); len(tls) != 1 {
		t.Errorf("tls_config: expected 1, got %d", len(tls))
	}
}

// TestSettingsCov3_Read_NoAuthClearsBlocks covers the else branches that clear
// oauth_settings and ldap_settings when the API returns neither block.
func TestSettingsCov3_Read_NoAuthClearsBlocks(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/settings", RespondJSON(http.StatusOK, map[string]interface{}{
		"authenticationMethod": 1,
		"snapshotInterval":     "5m",
	}))

	r := resourceSettings()
	d := r.TestResourceData()
	d.SetId("portainer-settings")

	if err := rcRead(r, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	if oauthList := d.Get("oauth_settings").([]interface{}); len(oauthList) != 0 {
		t.Errorf("oauth_settings: expected cleared, got %d entries", len(oauthList))
	}
	if ldapList := d.Get("ldap_settings").([]interface{}); len(ldapList) != 0 {
		t.Errorf("ldap_settings: expected cleared, got %d entries", len(ldapList))
	}
}
