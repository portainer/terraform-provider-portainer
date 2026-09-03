package internal

import (
	"net/http"
	"testing"
)

func mockSystemEndpoints(t *testing.T, adminStatus int) *MockServer {
	t.Helper()
	mock := NewMockServer(t)
	mock.On("GET", "/system/status", RespondJSON(http.StatusOK, map[string]interface{}{
		"InstanceID": "abc-123", "Version": "2.45.0",
	}))
	mock.On("GET", "/system/version", RespondJSON(http.StatusOK, map[string]interface{}{
		"ServerVersion": "2.45.0", "ServerEdition": "CE", "DatabaseVersion": "150",
		"LatestVersion": "2.46.0", "UpdateAvailable": true, "VersionSupport": "LTS",
	}))
	mock.On("GET", "/system/info", RespondJSON(http.StatusOK, map[string]interface{}{
		"agents": 3, "edgeAgents": 7, "platform": "docker",
	}))
	mock.On("GET", "/system/nodes", RespondJSON(http.StatusOK, map[string]interface{}{"nodes": 12}))
	if adminStatus == http.StatusNoContent {
		mock.On("GET", "/users/admin/check", RespondString(http.StatusNoContent, "", ""))
	} else {
		mock.On("GET", "/users/admin/check",
			RespondString(adminStatus, "application/json", `{"message":"no admin"}`))
	}
	return mock
}

// TestDataSourceSystem_HappyPath verifies the four system endpoints are folded
// into one data source and mapped correctly.
func TestDataSourceSystem_HappyPath(t *testing.T) {
	mock := mockSystemEndpoints(t, http.StatusNoContent)

	ds := dataSourceSystem()
	d := ds.TestResourceData()
	if err := rcRead(ds, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	for field, want := range map[string]interface{}{
		"instance_id": "abc-123", "version": "2.45.0",
		"server_version": "2.45.0", "server_edition": "CE", "database_version": "150",
		"latest_version": "2.46.0", "update_available": true, "version_support": "LTS",
		"agents": 3, "edge_agents": 7, "platform": "docker", "nodes": 12,
		"admin_initialized": true,
	} {
		if got := d.Get(field); got != want {
			t.Errorf("%s: expected %v, got %v", field, want, got)
		}
	}
	if d.Id() != "portainer-system-abc-123" {
		t.Errorf("id: got %q", d.Id())
	}
}

// TestDataSourceSystem_UninitialisedInstance verifies the admin check's 404 is
// read as "not initialised" rather than failing the read — that is the whole
// point of the flag.
func TestDataSourceSystem_UninitialisedInstance(t *testing.T) {
	mock := mockSystemEndpoints(t, http.StatusNotFound)

	ds := dataSourceSystem()
	d := ds.TestResourceData()
	if err := rcRead(ds, d, mock.Client()); err != nil {
		t.Fatalf("a 404 from the admin check must not fail the read: %v", err)
	}
	if got := d.Get("admin_initialized"); got != false {
		t.Errorf("admin_initialized: expected false, got %v", got)
	}
}

// TestDataSourceSystem_AdminCheckServerError verifies that a genuine failure of
// the admin check is still raised, so it cannot be mistaken for "no admin".
func TestDataSourceSystem_AdminCheckServerError(t *testing.T) {
	mock := mockSystemEndpoints(t, http.StatusInternalServerError)

	ds := dataSourceSystem()
	d := ds.TestResourceData()
	if err := rcRead(ds, d, mock.Client()); err == nil {
		t.Fatal("expected a 500 from the admin check to fail the read")
	}
}

// TestDataSourceSettingsPublic_HappyPath covers the unauthenticated settings
// endpoint used to discover how an instance expects to be authenticated.
func TestDataSourceSettingsPublic_HappyPath(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("GET", "/settings/public", RespondJSON(http.StatusOK, map[string]interface{}{
		"AuthenticationMethod": 3, "EnableEdgeComputeFeatures": true,
		"RequiredPasswordLength": 12, "RequiresSetupToken": false,
		"KubeconfigExpiry": "24h", "OAuthLoginURI": "https://sso.example.com/auth",
		"TeamSync": true,
	}))

	ds := dataSourceSettingsPublic()
	d := ds.TestResourceData()
	if err := rcRead(ds, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if got := d.Get("authentication_method"); got != 3 {
		t.Errorf("authentication_method: got %v", got)
	}
	if got := d.Get("enable_edge_compute_features"); got != true {
		t.Errorf("enable_edge_compute_features: got %v", got)
	}
	if got := d.Get("oauth_login_uri"); got != "https://sso.example.com/auth" {
		t.Errorf("oauth_login_uri: got %v", got)
	}
}

// TestDataSourceMOTD_HappyPath verifies the message of the day, whose Hash is
// []byte upstream and therefore arrives base64-encoded.
func TestDataSourceMOTD_HappyPath(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("GET", "/motd", RespondJSON(http.StatusOK, map[string]interface{}{
		"Title": "Maintenance", "Message": "Upgrading on Sunday", "Style": "color: red",
		"Hash": []byte("abc"),
	}))

	ds := dataSourceMOTD()
	d := ds.TestResourceData()
	if err := rcRead(ds, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if got := d.Get("title"); got != "Maintenance" {
		t.Errorf("title: got %v", got)
	}
	if got := d.Get("hash"); got == "" {
		t.Error("hash should be carried through as the opaque string it is")
	}
}

// TestDataSourceUserAccess_CurrentUser verifies that without a user_id the data
// source resolves the account the provider authenticates as, then follows it
// with the membership and effective-access lookups.
func TestDataSourceUserAccess_CurrentUser(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("GET", "/users/me", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id": 5, "Username": "ci", "Role": 2,
	}))
	mock.On("GET", "/users/5/memberships", RespondJSON(http.StatusOK, []map[string]interface{}{
		{"Id": 1, "TeamID": 3, "Role": 2},
	}))
	mock.On("GET", "/users/5/effective-access", RespondJSON(http.StatusOK, []map[string]interface{}{
		{
			"endpointId": 9, "endpointName": "prod", "groupId": 2, "groupName": "production",
			"teamId": 3, "teamName": "platform", "roleId": 1, "roleName": "Environment administrator",
			"rolePriority": 1, "accessLocation": "group",
		},
	}))

	ds := dataSourceUserAccess()
	d := ds.TestResourceData()
	if err := rcRead(ds, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	if got := d.Get("username"); got != "ci" {
		t.Errorf("username: got %v", got)
	}
	if got := d.Get("user_id"); got != 5 {
		t.Errorf("user_id should be resolved from /users/me, got %v", got)
	}
	if m := d.Get("memberships").([]interface{}); len(m) != 1 {
		t.Fatalf("memberships: expected 1 entry, got %d", len(m))
	}
	access := d.Get("effective_access").([]interface{})
	if len(access) != 1 {
		t.Fatalf("effective_access: expected 1 entry, got %d", len(access))
	}
	a := access[0].(map[string]interface{})
	if a["endpoint_name"] != "prod" || a["access_location"] != "group" || a["team_name"] != "platform" {
		t.Errorf("effective_access entry mismatch: %v", a)
	}
}

// TestDataSourceUserAccess_ExplicitUser verifies an explicit user_id looks the
// user up directly instead of resolving the caller.
func TestDataSourceUserAccess_ExplicitUser(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("GET", "/users/8", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id": 8, "Username": "alice", "Role": 1,
	}))
	mock.On("GET", "/users/8/memberships", RespondJSON(http.StatusOK, []map[string]interface{}{}))
	mock.On("GET", "/users/8/effective-access", RespondJSON(http.StatusOK, []map[string]interface{}{}))

	ds := dataSourceUserAccess()
	d := ds.TestResourceData()
	_ = d.Set("user_id", 8)

	if err := rcRead(ds, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if mock.FindRequest("GET", "/users/me") != nil {
		t.Error("an explicit user_id must not fall back to /users/me")
	}
	if got := d.Get("username"); got != "alice" {
		t.Errorf("username: got %v", got)
	}
}
