package internal

import (
	"net/http"
	"testing"
)

// resource_settings.go drives a single Portainer endpoint, /settings, with PUT
// for both Create and Update (resourceSettingsApply) and GET for Read. The bulk
// of the resource is field mapping (73 schema fields) between Terraform state
// and the SettingsPayload struct, so these tests target a representative subset
// plus the structure of the nested OAuth and LDAP blocks rather than every field.
//
// Important payload key quirks (from the struct tags in resource_settings.go):
//   - Most scalars use camelCase JSON keys (authenticationMethod, snapshotInterval,
//     edgeAgentCheckinInterval, enableEdgeComputeFeatures, ...).
//   - A few use PascalCase (EnableTelemetry, EdgePortainerURL).
//   - The nested OAuth block serializes under "oauthSettings" with PascalCase
//     inner keys (ClientID, AccessTokenURI, ...).
//   - The nested LDAP block serializes under the all-lowercase key "ldapsettings".
//   - Every field carries omitempty, so zero values (0, false, "") are dropped.

// TestSettingsCreate_HappyPath verifies that resourceSettingsApply issues a PUT
// to /settings, serializes a representative subset of scalar fields with the
// correct JSON keys, and assigns the fixed resource ID.
func TestSettingsCreate_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("PUT", "/settings", RespondJSON(http.StatusOK, map[string]interface{}{
		"authenticationMethod": 1,
	}))

	r := resourceSettings()
	d := r.TestResourceData()
	_ = d.Set("edge_portainer_url", "https://portainer.example.com")
	_ = d.Set("authentication_method", 1)
	_ = d.Set("enable_telemetry", true)
	_ = d.Set("logo_url", "https://example.com/logo.png")
	_ = d.Set("snapshot_interval", "5m")
	_ = d.Set("templates_url", "https://example.com/templates.json")
	_ = d.Set("enable_edge_compute_features", true)
	_ = d.Set("user_session_timeout", "8h")
	_ = d.Set("edge_agent_checkin_interval", 30)
	_ = d.Set("trust_on_first_connect", true)

	if err := rcCreate(r, d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	if d.Id() != "portainer-settings" {
		t.Errorf("expected ID %q, got %q", "portainer-settings", d.Id())
	}

	put := mock.FindRequest("PUT", "/settings")
	if put == nil {
		t.Fatal("expected a PUT to /settings")
	}
	var payload map[string]interface{}
	if err := put.DecodeJSON(&payload); err != nil {
		t.Fatalf("failed to decode PUT body: %v", err)
	}

	// PascalCase keys.
	if got := payload["EdgePortainerURL"]; got != "https://portainer.example.com" {
		t.Errorf("EdgePortainerURL: expected URL, got %v", got)
	}
	if got := payload["EnableTelemetry"]; got != true {
		t.Errorf("EnableTelemetry: expected true, got %v", got)
	}
	// camelCase keys. JSON numbers decode as float64.
	if got := payload["authenticationMethod"]; got != float64(1) {
		t.Errorf("authenticationMethod: expected 1, got %v", got)
	}
	if got := payload["logoURL"]; got != "https://example.com/logo.png" {
		t.Errorf("logoURL: expected logo URL, got %v", got)
	}
	if got := payload["snapshotInterval"]; got != "5m" {
		t.Errorf("snapshotInterval: expected 5m, got %v", got)
	}
	if got := payload["templatesURL"]; got != "https://example.com/templates.json" {
		t.Errorf("templatesURL: expected templates URL, got %v", got)
	}
	if got := payload["enableEdgeComputeFeatures"]; got != true {
		t.Errorf("enableEdgeComputeFeatures: expected true, got %v", got)
	}
	if got := payload["userSessionTimeout"]; got != "8h" {
		t.Errorf("userSessionTimeout: expected 8h, got %v", got)
	}
	if got := payload["edgeAgentCheckinInterval"]; got != float64(30) {
		t.Errorf("edgeAgentCheckinInterval: expected 30, got %v", got)
	}
	if got := payload["trustOnFirstConnect"]; got != true {
		t.Errorf("trustOnFirstConnect: expected true, got %v", got)
	}
}

// TestSettingsCreate_OAuthBlock verifies the nested oauth_settings TypeList(MaxItems:1)
// is serialized into the "oauthSettings" object with PascalCase inner keys,
// including the deeply nested team_memberships block.
func TestSettingsCreate_OAuthBlock(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("PUT", "/settings", RespondJSON(http.StatusOK, map[string]interface{}{}))

	r := resourceSettings()
	d := r.TestResourceData()
	_ = d.Set("authentication_method", 3)
	_ = d.Set("oauth_settings", []interface{}{
		map[string]interface{}{
			"access_token_uri":                "https://idp.example.com/token",
			"auth_style":                      2,
			"authorization_uri":               "https://idp.example.com/authorize",
			"client_id":                       "my-client-id",
			"client_secret":                   "super-secret",
			"default_team_id":                 4,
			"oauth_auto_create_users":         true,
			"sso":                             true,
			"scopes":                          "openid profile",
			"user_identifier":                 "email",
			"microsoft_tenant_id":             "tenant-123",
			"oauth_auto_map_team_memberships": true,
			"team_memberships": []interface{}{
				map[string]interface{}{
					"oauth_claim_name":              "groups",
					"admin_auto_populate":           true,
					"admin_group_claims_regex_list": []interface{}{"^admin-.*$"},
					"oauth_claim_mappings": []interface{}{
						map[string]interface{}{
							"claim_val_regex": "^dev-.*$",
							"team":            7,
						},
					},
				},
			},
		},
	})

	if err := rcCreate(r, d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	put := mock.FindRequest("PUT", "/settings")
	if put == nil {
		t.Fatal("expected a PUT to /settings")
	}
	var payload map[string]interface{}
	if err := put.DecodeJSON(&payload); err != nil {
		t.Fatalf("failed to decode PUT body: %v", err)
	}

	oauthRaw, ok := payload["oauthSettings"]
	if !ok {
		t.Fatalf("expected oauthSettings in payload, got keys: %v", payload)
	}
	oauth, ok := oauthRaw.(map[string]interface{})
	if !ok {
		t.Fatalf("oauthSettings: expected object, got %T", oauthRaw)
	}

	if got := oauth["AccessTokenURI"]; got != "https://idp.example.com/token" {
		t.Errorf("oauth.AccessTokenURI: got %v", got)
	}
	if got := oauth["AuthStyle"]; got != float64(2) {
		t.Errorf("oauth.AuthStyle: expected 2, got %v", got)
	}
	if got := oauth["ClientID"]; got != "my-client-id" {
		t.Errorf("oauth.ClientID: got %v", got)
	}
	if got := oauth["ClientSecret"]; got != "super-secret" {
		t.Errorf("oauth.ClientSecret: got %v", got)
	}
	if got := oauth["DefaultTeamID"]; got != float64(4) {
		t.Errorf("oauth.DefaultTeamID: expected 4, got %v", got)
	}
	if got := oauth["SSO"]; got != true {
		t.Errorf("oauth.SSO: expected true, got %v", got)
	}
	if got := oauth["Scopes"]; got != "openid profile" {
		t.Errorf("oauth.Scopes: got %v", got)
	}
	if got := oauth["MicrosoftTenantID"]; got != "tenant-123" {
		t.Errorf("oauth.MicrosoftTenantID: got %v", got)
	}

	// Nested team_memberships block.
	tmRaw, ok := oauth["TeamMemberships"]
	if !ok {
		t.Fatalf("expected TeamMemberships in oauthSettings, got keys: %v", oauth)
	}
	tm, ok := tmRaw.(map[string]interface{})
	if !ok {
		t.Fatalf("TeamMemberships: expected object, got %T", tmRaw)
	}
	if got := tm["OAuthClaimName"]; got != "groups" {
		t.Errorf("tm.OAuthClaimName: got %v", got)
	}
	if got := tm["AdminAutoPopulate"]; got != true {
		t.Errorf("tm.AdminAutoPopulate: expected true, got %v", got)
	}
	regexList, ok := tm["AdminGroupClaimsRegexList"].([]interface{})
	if !ok || len(regexList) != 1 || regexList[0] != "^admin-.*$" {
		t.Errorf("tm.AdminGroupClaimsRegexList: got %v", tm["AdminGroupClaimsRegexList"])
	}
	mappings, ok := tm["OAuthClaimMappings"].([]interface{})
	if !ok || len(mappings) != 1 {
		t.Fatalf("tm.OAuthClaimMappings: expected 1 mapping, got %v", tm["OAuthClaimMappings"])
	}
	mapping := mappings[0].(map[string]interface{})
	if got := mapping["ClaimValRegex"]; got != "^dev-.*$" {
		t.Errorf("mapping.ClaimValRegex: got %v", got)
	}
	if got := mapping["Team"]; got != float64(7) {
		t.Errorf("mapping.Team: expected 7, got %v", got)
	}
}

// TestSettingsCreate_LDAPBlock verifies the nested ldap_settings block is
// serialized under the all-lowercase "ldapsettings" key, with its search,
// group-search and TLS sub-blocks intact.
func TestSettingsCreate_LDAPBlock(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("PUT", "/settings", RespondJSON(http.StatusOK, map[string]interface{}{}))

	r := resourceSettings()
	d := r.TestResourceData()
	_ = d.Set("authentication_method", 2)
	_ = d.Set("ldap_settings", []interface{}{
		map[string]interface{}{
			"anonymous_mode":    false,
			"auto_create_users": true,
			"password":          "ldap-secret",
			"reader_dn":         "cn=reader,dc=example,dc=com",
			"start_tls":         true,
			"url":               "ldap://ldap.example.com:389",
			"search_settings": []interface{}{
				map[string]interface{}{
					"base_dn":             "dc=example,dc=com",
					"filter":              "(objectClass=person)",
					"user_name_attribute": "uid",
				},
			},
			"group_search_settings": []interface{}{
				map[string]interface{}{
					"group_attribute": "member",
					"group_base_dn":   "ou=groups,dc=example,dc=com",
					"group_filter":    "(objectClass=groupOfNames)",
				},
			},
			"tls_config": []interface{}{
				map[string]interface{}{
					"tls":             true,
					"tls_ca_cert":     "ca-pem",
					"tls_skip_verify": true,
				},
			},
		},
	})

	if err := rcCreate(r, d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	put := mock.FindRequest("PUT", "/settings")
	if put == nil {
		t.Fatal("expected a PUT to /settings")
	}
	var payload map[string]interface{}
	if err := put.DecodeJSON(&payload); err != nil {
		t.Fatalf("failed to decode PUT body: %v", err)
	}

	// Note the all-lowercase JSON key.
	ldapRaw, ok := payload["ldapsettings"]
	if !ok {
		t.Fatalf("expected ldapsettings in payload, got keys: %v", payload)
	}
	ldap, ok := ldapRaw.(map[string]interface{})
	if !ok {
		t.Fatalf("ldapsettings: expected object, got %T", ldapRaw)
	}

	if got := ldap["AutoCreateUsers"]; got != true {
		t.Errorf("ldap.AutoCreateUsers: expected true, got %v", got)
	}
	if got := ldap["Password"]; got != "ldap-secret" {
		t.Errorf("ldap.Password: got %v", got)
	}
	if got := ldap["ReaderDN"]; got != "cn=reader,dc=example,dc=com" {
		t.Errorf("ldap.ReaderDN: got %v", got)
	}
	if got := ldap["URL"]; got != "ldap://ldap.example.com:389" {
		t.Errorf("ldap.URL: got %v", got)
	}

	search, ok := ldap["SearchSettings"].([]interface{})
	if !ok || len(search) != 1 {
		t.Fatalf("ldap.SearchSettings: expected 1 entry, got %v", ldap["SearchSettings"])
	}
	s := search[0].(map[string]interface{})
	if got := s["BaseDN"]; got != "dc=example,dc=com" {
		t.Errorf("search.BaseDN: got %v", got)
	}
	if got := s["UserNameAttribute"]; got != "uid" {
		t.Errorf("search.UserNameAttribute: got %v", got)
	}

	groupSearch, ok := ldap["GroupSearchSettings"].([]interface{})
	if !ok || len(groupSearch) != 1 {
		t.Fatalf("ldap.GroupSearchSettings: expected 1 entry, got %v", ldap["GroupSearchSettings"])
	}
	gs := groupSearch[0].(map[string]interface{})
	if got := gs["GroupBaseDN"]; got != "ou=groups,dc=example,dc=com" {
		t.Errorf("groupSearch.GroupBaseDN: got %v", got)
	}

	tlsRaw, ok := ldap["TLSConfig"]
	if !ok {
		t.Fatalf("expected TLSConfig in ldapsettings, got keys: %v", ldap)
	}
	tls := tlsRaw.(map[string]interface{})
	if got := tls["TLS"]; got != true {
		t.Errorf("tls.TLS: expected true, got %v", got)
	}
	if got := tls["TLSCACert"]; got != "ca-pem" {
		t.Errorf("tls.TLSCACert: got %v", got)
	}
	if got := tls["TLSSkipVerify"]; got != true {
		t.Errorf("tls.TLSSkipVerify: expected true, got %v", got)
	}
}

// TestSettingsRead_HappyPath verifies the GET /settings response is mapped back
// into Terraform state for a representative subset of scalar fields, the
// black_listed_labels list, and the nested oauth_settings / ldap_settings blocks.
func TestSettingsRead_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/settings", RespondJSON(http.StatusOK, map[string]interface{}{
		"EdgePortainerURL":          "https://portainer.example.com",
		"authenticationMethod":      3,
		"logoURL":                   "https://example.com/logo.png",
		"snapshotInterval":          "10m",
		"templatesURL":              "https://example.com/templates.json",
		"enableEdgeComputeFeatures": true,
		"enforceEdgeID":             true,
		"userSessionTimeout":        "8h",
		"kubeconfigExpiry":          "24h",
		"kubectlShellImage":         "portainer/kubectl-shell",
		"helmRepositoryURL":         "https://charts.example.com",
		"trustOnFirstConnect":       true,
		"edgeAgentCheckinInterval":  60,
		"DisableKubeShell":          true,
		"DisplayDonationHeader":     true,
		"blackListedLabels": []map[string]interface{}{
			{"name": "secret", "value": "hidden"},
		},
		"oauthSettings": map[string]interface{}{
			"ClientID":      "client-x",
			"AuthStyle":     1,
			"DefaultTeamID": 2,
			"SSO":           true,
			"Scopes":        "openid",
		},
		"ldapsettings": map[string]interface{}{
			"URL":             "ldap://ldap.example.com",
			"AutoCreateUsers": true,
			"ReaderDN":        "cn=reader",
		},
	}))

	r := resourceSettings()
	d := r.TestResourceData()
	d.SetId("portainer-settings")

	if err := rcRead(r, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	if d.Id() != "portainer-settings" {
		t.Errorf("expected ID %q, got %q", "portainer-settings", d.Id())
	}

	// Representative scalar fields.
	if got := d.Get("edge_portainer_url"); got != "https://portainer.example.com" {
		t.Errorf("edge_portainer_url: got %v", got)
	}
	if got := d.Get("authentication_method"); got != 3 {
		t.Errorf("authentication_method: expected 3, got %v", got)
	}
	if got := d.Get("snapshot_interval"); got != "10m" {
		t.Errorf("snapshot_interval: got %v", got)
	}
	if got := d.Get("enable_edge_compute_features"); got != true {
		t.Errorf("enable_edge_compute_features: expected true, got %v", got)
	}
	if got := d.Get("enforce_edge_id"); got != true {
		t.Errorf("enforce_edge_id: expected true, got %v", got)
	}
	if got := d.Get("kubeconfig_expiry"); got != "24h" {
		t.Errorf("kubeconfig_expiry: got %v", got)
	}
	if got := d.Get("edge_agent_checkin_interval"); got != 60 {
		t.Errorf("edge_agent_checkin_interval: expected 60, got %v", got)
	}
	if got := d.Get("disable_kube_shell"); got != true {
		t.Errorf("disable_kube_shell: expected true, got %v", got)
	}
	if got := d.Get("display_donation_header"); got != true {
		t.Errorf("display_donation_header: expected true, got %v", got)
	}

	// black_listed_labels list mapping.
	labels := d.Get("black_listed_labels").([]interface{})
	if len(labels) != 1 {
		t.Fatalf("black_listed_labels: expected 1 entry, got %d", len(labels))
	}
	label := labels[0].(map[string]interface{})
	if label["name"] != "secret" || label["value"] != "hidden" {
		t.Errorf("black_listed_labels[0]: got %v", label)
	}

	// oauth_settings block mapping.
	oauthList := d.Get("oauth_settings").([]interface{})
	if len(oauthList) != 1 {
		t.Fatalf("oauth_settings: expected 1 block, got %d", len(oauthList))
	}
	oauth := oauthList[0].(map[string]interface{})
	if got := oauth["client_id"]; got != "client-x" {
		t.Errorf("oauth.client_id: got %v", got)
	}
	if got := oauth["auth_style"]; got != 1 {
		t.Errorf("oauth.auth_style: expected 1, got %v", got)
	}
	if got := oauth["sso"]; got != true {
		t.Errorf("oauth.sso: expected true, got %v", got)
	}

	// ldap_settings block mapping.
	ldapList := d.Get("ldap_settings").([]interface{})
	if len(ldapList) != 1 {
		t.Fatalf("ldap_settings: expected 1 block, got %d", len(ldapList))
	}
	ldap := ldapList[0].(map[string]interface{})
	if got := ldap["url"]; got != "ldap://ldap.example.com" {
		t.Errorf("ldap.url: got %v", got)
	}
	if got := ldap["auto_create_users"]; got != true {
		t.Errorf("ldap.auto_create_users: expected true, got %v", got)
	}
	if got := ldap["reader_dn"]; got != "cn=reader" {
		t.Errorf("ldap.reader_dn: got %v", got)
	}
}

// TestSettingsUpdate_ReusesApply verifies that Update routes through the same
// PUT /settings path as Create (Create and Update both bind to
// resourceSettingsApply) and serializes the changed scalar fields.
func TestSettingsUpdate_ReusesApply(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("PUT", "/settings", RespondJSON(http.StatusOK, map[string]interface{}{}))

	r := resourceSettings()
	d := r.TestResourceData()
	d.SetId("portainer-settings")
	_ = d.Set("authentication_method", 2)
	_ = d.Set("snapshot_interval", "15m")

	if err := rcUpdate(r, d, mock.Client()); err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	put := mock.FindRequest("PUT", "/settings")
	if put == nil {
		t.Fatal("expected a PUT to /settings on Update")
	}
	var payload map[string]interface{}
	if err := put.DecodeJSON(&payload); err != nil {
		t.Fatalf("failed to decode PUT body: %v", err)
	}
	if got := payload["authenticationMethod"]; got != float64(2) {
		t.Errorf("authenticationMethod: expected 2, got %v", got)
	}
	if got := payload["snapshotInterval"]; got != "15m" {
		t.Errorf("snapshotInterval: expected 15m, got %v", got)
	}
	if d.Id() != "portainer-settings" {
		t.Errorf("expected ID preserved, got %q", d.Id())
	}
}

// TestSettingsCreate_HTTPError verifies that a 4xx/5xx response from PUT /settings
// is surfaced as an error and the resource ID is not set.
func TestSettingsCreate_HTTPError(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("PUT", "/settings", RespondString(
		http.StatusBadRequest, "application/json",
		`{"message":"invalid settings"}`,
	))

	r := resourceSettings()
	d := r.TestResourceData()
	_ = d.Set("authentication_method", 1)

	err := rcCreate(r, d, mock.Client())
	if err == nil {
		t.Fatal("expected error on HTTP 400, got nil")
	}
	if d.Id() != "" {
		t.Errorf("expected empty ID after error, got %q", d.Id())
	}
}

// TestSettingsRead_HTTPError verifies that a non-200 response from GET /settings
// is surfaced as an error.
func TestSettingsRead_HTTPError(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/settings", RespondString(
		http.StatusInternalServerError, "application/json",
		`{"message":"boom"}`,
	))

	r := resourceSettings()
	d := r.TestResourceData()
	d.SetId("portainer-settings")

	if err := rcRead(r, d, mock.Client()); err == nil {
		t.Fatal("expected error on HTTP 500, got nil")
	}
}

// TestSettingsDelete_ClearsID verifies that Delete is effectively a no-op that
// only removes the resource from state (Portainer settings cannot be deleted);
// it must not panic or issue any HTTP request.
func TestSettingsDelete_ClearsID(t *testing.T) {
	mock := NewMockServer(t)

	r := resourceSettings()
	d := r.TestResourceData()
	d.SetId("portainer-settings")

	if err := rcDelete(r, d, mock.Client()); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	if d.Id() != "" {
		t.Errorf("expected ID cleared after Delete, got %q", d.Id())
	}
	if len(mock.Requests()) != 0 {
		t.Errorf("expected no HTTP requests during Delete, got %d", len(mock.Requests()))
	}
}
