package internal

import (
	"fmt"
	"net/http"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// Unit tests of the user/team data sources of batch H, run through real
// Terraform against a MockServer.

func hUserDSTest(t *testing.T, mock *MockServer, config string, check resource.TestCheckFunc) {
	t.Helper()
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps:                    []resource.TestStep{{Config: mockProviderConfig(mock) + config, Check: check}},
	})
}

func hUserDSTestError(t *testing.T, mock *MockServer, config string, want *regexp.Regexp) {
	t.Helper()
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps:                    []resource.TestStep{{Config: mockProviderConfig(mock) + config, ExpectError: want}},
	})
}

func hUserDSBoom() http.HandlerFunc {
	return RespondString(http.StatusInternalServerError, "application/json", `{"message":"boom"}`)
}

// --- portainer_user ---

func TestUserDataSource_HappyPath(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	// As live on 3.0.1: the list leaves the authentication method fields
	// out, GET /users/{id} has them.
	mock.On("GET", "/users", RespondJSON(http.StatusOK, []map[string]interface{}{
		{"Id": 1, "Username": "admin", "Role": 1},
		{"Id": 5, "Username": "alice", "Role": 2},
		{"Id": 6, "Username": "bob", "Role": 2},
	}))
	mock.On("GET", "/users/5", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id": 5, "Username": "alice", "Role": 2, "CreatedAuthenticationMethod": 3,
	}))
	hUserDSTest(t, mock, `data "portainer_user" "u" { username = "alice" }`, resource.ComposeTestCheckFunc(
		resource.TestCheckResourceAttr("data.portainer_user.u", "id", "5"),
		resource.TestCheckResourceAttr("data.portainer_user.u", "role", "2"),
		resource.TestCheckResourceAttr("data.portainer_user.u", "created_authentication_method", "oauth"),
		resource.TestCheckResourceAttr("data.portainer_user.u", "last_authentication_method", ""),
	))
}

func TestUserDataSource_Pre30DetailMissing(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("GET", "/users", RespondJSON(http.StatusOK, []map[string]interface{}{{"Id": 5, "Username": "alice", "Role": 2}}))
	// GET /users/5 is not mocked (404): the lookup still succeeds.
	hUserDSTest(t, mock, `data "portainer_user" "u" { username = "alice" }`, resource.ComposeTestCheckFunc(
		resource.TestCheckResourceAttr("data.portainer_user.u", "id", "5"),
		resource.TestCheckResourceAttr("data.portainer_user.u", "created_authentication_method", ""),
	))
}

func TestUserDataSource_NotFound(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("GET", "/users", RespondJSON(http.StatusOK, []map[string]interface{}{{"Id": 1, "Username": "admin", "Role": 1}}))
	hUserDSTestError(t, mock, `data "portainer_user" "u" { username = "ghost" }`, regexp.MustCompile(`user ghost not found`))
}

func TestUserDataSource_HTTPError(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("GET", "/users", hUserDSBoom())
	hUserDSTestError(t, mock, `data "portainer_user" "u" { username = "alice" }`, regexp.MustCompile(`failed to list users`))
}

// --- portainer_team ---

func TestTeamDataSource_HappyPath(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("GET", "/teams", RespondJSON(http.StatusOK, []map[string]interface{}{
		{"Id": 1, "Name": "developers"},
		{"Id": 2, "Name": "ops"},
	}))
	hUserDSTest(t, mock, `data "portainer_team" "t" { name = "ops" }`,
		resource.TestCheckResourceAttr("data.portainer_team.t", "id", "2"))
}

func TestTeamDataSource_NotFound(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("GET", "/teams", RespondJSON(http.StatusOK, []map[string]interface{}{{"Id": 1, "Name": "developers"}}))
	hUserDSTestError(t, mock, `data "portainer_team" "t" { name = "ghost" }`, regexp.MustCompile(`team ghost not found`))
}

func TestTeamDataSource_HTTPError(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("GET", "/teams", hUserDSBoom())
	hUserDSTestError(t, mock, `data "portainer_team" "t" { name = "ops" }`, regexp.MustCompile(`boom|500`))
}

// --- portainer_team_membership ---

func TestTeamMembershipDataSource_HappyPath(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("GET", "/team_memberships", RespondJSON(http.StatusOK, []map[string]interface{}{
		{"Id": 10, "TeamID": 1, "UserID": 5, "Role": 1},
		{"Id": 11, "TeamID": 2, "UserID": 5, "Role": 2},
		{"Id": 12, "TeamID": 2, "UserID": 6, "Role": 2},
	}))
	hUserDSTest(t, mock, `data "portainer_team_membership" "m" {
  team_id = 2
  user_id = 5
}`, resource.ComposeTestCheckFunc(
		resource.TestCheckResourceAttr("data.portainer_team_membership.m", "id", "11"),
		resource.TestCheckResourceAttr("data.portainer_team_membership.m", "role", "2"),
	))
}

func TestTeamMembershipDataSource_NotFound(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("GET", "/team_memberships", RespondJSON(http.StatusOK, []map[string]interface{}{{"Id": 10, "TeamID": 1, "UserID": 5, "Role": 1}}))
	hUserDSTestError(t, mock, `data "portainer_team_membership" "m" {
  team_id = 99
  user_id = 99
}`, regexp.MustCompile(`team membership not found for team_id 99 and user_id 99`))
}

func TestTeamMembershipDataSource_HTTPError(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("GET", "/team_memberships", hUserDSBoom())
	hUserDSTestError(t, mock, `data "portainer_team_membership" "m" {
  team_id = 1
  user_id = 1
}`, regexp.MustCompile(`boom|500`))
}

// --- portainer_team_memberships ---

func TestTeamMembershipsDataSource_Leaders(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("GET", "/teams/1/memberships", RespondJSON(http.StatusOK, []map[string]interface{}{
		{"Id": 1, "UserID": 5, "Role": 1},
		{"Id": 2, "UserID": 6, "Role": 2},
		{"Id": 3, "UserID": 7, "Role": 1},
	}))
	n := "data.portainer_team_memberships.m"
	hUserDSTest(t, mock, `data "portainer_team_memberships" "m" { team_id = 1 }`, resource.ComposeTestCheckFunc(
		resource.TestCheckResourceAttr(n, "id", "1/memberships"),
		resource.TestCheckResourceAttr(n, "memberships.#", "3"),
		resource.TestCheckResourceAttr(n, "memberships.1.user_id", "6"),
		resource.TestCheckResourceAttr(n, "memberships.1.role", "2"),
		resource.TestCheckResourceAttr(n, "leader_user_ids.#", "2"),
		resource.TestCheckResourceAttr(n, "leader_user_ids.0", "5"),
		resource.TestCheckResourceAttr(n, "leader_user_ids.1", "7"),
	))
}

func TestTeamMembershipsDataSource_EmptyTeam(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("GET", "/teams/1/memberships", RespondJSON(http.StatusOK, []map[string]interface{}{}))
	n := "data.portainer_team_memberships.m"
	hUserDSTest(t, mock, `data "portainer_team_memberships" "m" { team_id = 1 }`, resource.ComposeTestCheckFunc(
		resource.TestCheckResourceAttr(n, "memberships.#", "0"),
		resource.TestCheckResourceAttr(n, "leader_user_ids.#", "0"),
	))
}

// --- portainer_user_activity ---

func hUserDSActivityMock(mock *MockServer) {
	mock.On("GET", "/useractivity/logs", RespondJSON(http.StatusOK, map[string]interface{}{
		"logs": []map[string]interface{}{
			{"id": 1, "timestamp": 1700000000, "username": "alice", "action": "login", "context": "endpoint1"},
			{"id": 2, "timestamp": 1700001000, "username": "bob", "action": "logout", "context": "endpoint2"},
		},
		"totalCount": 2,
	}))
}

// The SDKv2 defaults still apply: log_type activity, limit=100 sent, offset
// 0 and sort_desc false left out of the query.
func TestUserActivityDataSource_Defaults(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	hUserDSActivityMock(mock)
	n := "data.portainer_user_activity.a"
	hUserDSTest(t, mock, `data "portainer_user_activity" "a" {}`, resource.ComposeTestCheckFunc(
		resource.TestCheckResourceAttr(n, "log_type", "activity"),
		resource.TestCheckResourceAttr(n, "limit", "100"),
		resource.TestCheckResourceAttr(n, "offset", "0"),
		resource.TestCheckResourceAttr(n, "sort_desc", "false"),
		resource.TestCheckResourceAttr(n, "activity_logs.#", "2"),
		resource.TestCheckResourceAttr(n, "activity_logs.1.username", "bob"),
		resource.TestCheckResourceAttr(n, "auth_logs.#", "0"),
		resource.TestCheckResourceAttr(n, "total_count", "2"),
		resource.TestCheckResourceAttrSet(n, "id"),
		func(_ *terraform.State) error {
			req := mock.FindRequest("GET", "/useractivity/logs")
			if req == nil {
				return fmt.Errorf("expected GET /useractivity/logs")
			}
			if req.Query != "limit=100" {
				return fmt.Errorf("query: expected %q, got %q", "limit=100", req.Query)
			}
			return nil
		},
	))
}

func TestUserActivityDataSource_AllFilters(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	hUserDSActivityMock(mock)
	hUserDSTest(t, mock, `data "portainer_user_activity" "a" {
  log_type  = "activity"
  offset    = 5
  limit     = 10
  before    = 100
  after     = 50
  sort_by   = "Timestamp"
  sort_desc = true
  keyword   = "foo"
  username  = ["alice", "bob"]
  context   = ["ctx1"]
}`, func(_ *terraform.State) error {
		req := mock.FindRequest("GET", "/useractivity/logs")
		if req == nil {
			return fmt.Errorf("expected GET /useractivity/logs")
		}
		for _, want := range []string{
			"offset=5", "limit=10", "before=100", "after=50",
			"sortBy=Timestamp", "sortDesc=true", "keyword=foo",
			"username=alice%2Cbob", "context=ctx1",
		} {
			if !strings.Contains(req.Query, want) {
				return fmt.Errorf("query %q missing %q", req.Query, want)
			}
		}
		return nil
	})
}

func TestUserActivityDataSource_Auth(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("GET", "/useractivity/authlogs", RespondJSON(http.StatusOK, []map[string]interface{}{
		{"id": 1, "timestamp": 1700000000, "username": "alice", "type": 1, "origin": "192.168.1.1", "context": 1},
	}))
	n := "data.portainer_user_activity.a"
	hUserDSTest(t, mock, `data "portainer_user_activity" "a" {
  log_type = "auth"
  username = ["ignored"]
}`, resource.ComposeTestCheckFunc(
		resource.TestCheckResourceAttr(n, "auth_logs.#", "1"),
		resource.TestCheckResourceAttr(n, "auth_logs.0.username", "alice"),
		resource.TestCheckResourceAttr(n, "auth_logs.0.origin", "192.168.1.1"),
		resource.TestCheckResourceAttr(n, "activity_logs.#", "0"),
		func(_ *terraform.State) error {
			// The username filter applies to activity logs only.
			if q := mock.FindRequest("GET", "/useractivity/authlogs").Query; strings.Contains(q, "username") {
				return fmt.Errorf("auth logs must not send the username filter: %q", q)
			}
			return nil
		},
	))
}

// TestUserActivityDataSource_AuthPaginated: Portainer BE 3.1 returns the auth
// logs as {"logs": [...], "totalCount": N} (the specs document a bare array,
// which TestUserActivityDataSource_Auth covers).
func TestUserActivityDataSource_AuthPaginated(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("GET", "/useractivity/authlogs", RespondJSON(http.StatusOK, map[string]interface{}{
		"logs": []map[string]interface{}{
			{"id": 7, "timestamp": 1795772315, "username": "admin", "type": 1, "origin": "192.168.65.1", "context": 1},
		},
		"totalCount": 42,
	}))
	n := "data.portainer_user_activity.a"
	hUserDSTest(t, mock, `data "portainer_user_activity" "a" { log_type = "auth" }`, resource.ComposeTestCheckFunc(
		resource.TestCheckResourceAttr(n, "auth_logs.#", "1"),
		resource.TestCheckResourceAttr(n, "auth_logs.0.id", "7"),
		resource.TestCheckResourceAttr(n, "auth_logs.0.username", "admin"),
		resource.TestCheckResourceAttr(n, "auth_logs.0.origin", "192.168.65.1"),
		resource.TestCheckResourceAttr(n, "total_count", "42"),
		resource.TestCheckResourceAttr(n, "activity_logs.#", "0"),
	))
}

func TestDecodeAuthLogs(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct {
		raw   string
		n     int
		total int64
	}{
		"array":  {`[{"id":1},{"id":2}]`, 2, 2},
		"object": {`{"logs":[{"id":1}],"totalCount":5}`, 1, 5},
		"empty":  {`{"logs":[],"totalCount":0}`, 0, 0},
		"null":   {`null`, 0, 0},
	} {
		logs, total, err := decodeAuthLogs([]byte(tc.raw))
		if err != nil || len(logs) != tc.n || total != tc.total {
			t.Errorf("%s: got %d logs, total %d, err %v; want %d, %d", name, len(logs), total, err, tc.n, tc.total)
		}
	}
	if _, _, err := decodeAuthLogs([]byte(`"x"`)); err == nil {
		t.Error("a string must not decode")
	}
}

func TestUserActivityDataSource_InvalidLogType(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	hUserDSTestError(t, mock, `data "portainer_user_activity" "a" { log_type = "bogus" }`, regexp.MustCompile(`invalid log_type "bogus"`))
}

func TestUserActivityDataSource_Errors(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, path, logType string
		handler             http.HandlerFunc
	}{
		{"activity-http", "/useractivity/logs", "activity", hUserDSBoom()},
		{"activity-decode", "/useractivity/logs", "activity", RespondString(http.StatusOK, "application/json", `{ not valid json`)},
		{"auth-http", "/useractivity/authlogs", "auth", hUserDSBoom()},
		{"auth-decode", "/useractivity/authlogs", "auth", RespondString(http.StatusOK, "application/json", `{ not an array`)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			mock := NewMockServer(t)
			mock.On("GET", tc.path, tc.handler)
			hUserDSTestError(t, mock, `data "portainer_user_activity" "a" { log_type = "`+tc.logType+`" }`,
				regexp.MustCompile(`failed to list user activity logs`))
		})
	}
}

// --- portainer_role ---

func hUserDSRolesMock(mock *MockServer) {
	mock.On("GET", "/roles", RespondJSON(http.StatusOK, []map[string]interface{}{
		{"Id": 1, "Name": "endpoint-administrator", "Description": "Full control", "Priority": 1},
		{"Id": 2, "Name": "helpdesk", "Description": "Read-only", "Priority": 2},
		{"Id": 3, "Name": "standard-user", "Description": "Standard", "Priority": 3},
	}))
}

func TestRoleDataSource_AllRoles(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	hUserDSRolesMock(mock)
	n := "data.portainer_role.r"
	hUserDSTest(t, mock, `data "portainer_role" "r" {}`, resource.ComposeTestCheckFunc(
		resource.TestCheckResourceAttr(n, "roles.#", "3"),
		resource.TestCheckResourceAttrSet(n, "id"),
	))
}

func TestRoleDataSource_FilterByName(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	hUserDSRolesMock(mock)
	n := "data.portainer_role.r"
	hUserDSTest(t, mock, `data "portainer_role" "r" { name = "helpdesk" }`, resource.ComposeTestCheckFunc(
		resource.TestCheckResourceAttr(n, "roles.#", "1"),
		resource.TestCheckResourceAttr(n, "roles.0.name", "helpdesk"),
		resource.TestCheckResourceAttr(n, "roles.0.description", "Read-only"),
		resource.TestCheckResourceAttr(n, "roles.0.priority", "2"),
		resource.TestCheckResourceAttr(n, "id", "2"),
	))
}

func TestRoleDataSource_FilterNoMatch(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	hUserDSRolesMock(mock)
	hUserDSTestError(t, mock, `data "portainer_role" "r" { name = "ghost" }`, regexp.MustCompile(`role with name "ghost" not found`))
}

func TestRoleDataSource_HTTPError(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("GET", "/roles", hUserDSBoom())
	hUserDSTestError(t, mock, `data "portainer_role" "r" {}`, regexp.MustCompile(`failed to list roles`))
}

// --- portainer_user_namespaces ---

// The nested map Portainer answers with is flattened and sorted, so two
// identical plans produce identical output.
func TestUserNamespacesDataSource_FlattensAndSorts(t *testing.T) {
	t.Parallel()
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
	n := "data.portainer_user_namespaces.n"
	hUserDSTest(t, mock, `data "portainer_user_namespaces" "n" { user_id = 7 }`, resource.ComposeTestCheckFunc(
		resource.TestCheckResourceAttr(n, "id", "portainer-user-namespaces-7"),
		resource.TestCheckResourceAttr(n, "namespaces.#", "3"),
		resource.TestCheckResourceAttr(n, "namespaces.0.endpoint_id", "1"),
		resource.TestCheckResourceAttr(n, "namespaces.0.namespace", "default"),
		resource.TestCheckResourceAttr(n, "namespaces.0.authorizations.#", "0"),
		resource.TestCheckResourceAttr(n, "namespaces.1.endpoint_id", "2"),
		resource.TestCheckResourceAttr(n, "namespaces.1.namespace", "apps"),
		resource.TestCheckResourceAttr(n, "namespaces.2.authorizations.#", "1"),
		resource.TestCheckResourceAttr(n, "namespaces.2.authorizations.0", "K8sAccessNamespaceRead"),
		resource.TestMatchResourceAttr(n, "raw", regexp.MustCompile(`K8sAccessNamespaceWrite`)),
	))
}

func TestUserNamespacesDataSource_Errors(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("GET", "/users/7/namespaces", hUserDSBoom())
	hUserDSTestError(t, mock, `data "portainer_user_namespaces" "n" { user_id = 7 }`, regexp.MustCompile(`failed to read the namespace authorizations of user 7`))

	bad := NewMockServer(t)
	bad.On("GET", "/users/7/namespaces", RespondString(http.StatusOK, "application/json", `[1]`))
	hUserDSTestError(t, bad, `data "portainer_user_namespaces" "n" { user_id = 7 }`, regexp.MustCompile(`failed to parse the namespace authorizations of user 7`))
}

// --- portainer_current_user_authorizations ---

func TestCurrentUserAuthorizationsDataSource_ReadsBothCalls(t *testing.T) {
	t.Parallel()
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
	n := "data.portainer_current_user_authorizations.c"
	hUserDSTest(t, mock, `data "portainer_current_user_authorizations" "c" { endpoint_id = 5 }`, resource.ComposeTestCheckFunc(
		resource.TestCheckResourceAttr(n, "id", "portainer-current-user-authorizations-5"),
		resource.TestCheckResourceAttr(n, "authorizations.#", "2"),
		resource.TestCheckResourceAttr(n, "authorizations.0", "DockerContainerList"),
		resource.TestCheckResourceAttr(n, "authorizations.1", "EndpointResourcesAccess"),
		resource.TestCheckResourceAttr(n, "namespace_authorizations.#", "1"),
		resource.TestCheckResourceAttr(n, "namespace_authorizations.0.namespace", "apps"),
		resource.TestCheckResourceAttr(n, "namespace_authorizations.0.authorizations.0", "K8sAccessNamespaceRead"),
	))
}

func TestCurrentUserAuthorizationsDataSource_EmptyForAdmins(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("GET", "/users/me/auth/5", RespondJSON(http.StatusOK, map[string]interface{}{"Authorizations": map[string]bool{}}))
	mock.On("GET", "/users/me/auth/5/namespaces", RespondJSON(http.StatusOK, map[string]interface{}{}))
	n := "data.portainer_current_user_authorizations.c"
	hUserDSTest(t, mock, `data "portainer_current_user_authorizations" "c" { endpoint_id = 5 }`, resource.ComposeTestCheckFunc(
		resource.TestCheckResourceAttr(n, "authorizations.#", "0"),
		resource.TestCheckResourceAttr(n, "namespace_authorizations.#", "0"),
	))
}

func TestCurrentUserAuthorizationsDataSource_NamespacesError(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("GET", "/users/me/auth/5", RespondJSON(http.StatusOK, map[string]interface{}{"Authorizations": map[string]bool{}}))
	mock.On("GET", "/users/me/auth/5/namespaces", hUserDSBoom())
	hUserDSTestError(t, mock, `data "portainer_current_user_authorizations" "c" { endpoint_id = 5 }`,
		regexp.MustCompile(`failed to read the current user's namespace authorizations on environment 5`))
}

// --- portainer_user_access ---

// Without a user_id the data source resolves the account the provider
// authenticates as, then follows it with the membership and effective-access
// lookups.
func TestUserAccessDataSource_CurrentUser(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("GET", "/users/me", RespondJSON(http.StatusOK, map[string]interface{}{"Id": 5, "Username": "ci", "Role": 2}))
	mock.On("GET", "/users/5/memberships", RespondJSON(http.StatusOK, []map[string]interface{}{{"Id": 1, "TeamID": 3, "Role": 2}}))
	mock.On("GET", "/users/5/effective-access", RespondJSON(http.StatusOK, []map[string]interface{}{
		{
			"endpointId": 9, "endpointName": "prod", "groupId": 2, "groupName": "production",
			"teamId": 3, "teamName": "platform", "roleId": 1, "roleName": "Environment administrator",
			"rolePriority": 1, "accessLocation": "group",
		},
	}))
	n := "data.portainer_user_access.a"
	hUserDSTest(t, mock, `data "portainer_user_access" "a" {}`, resource.ComposeTestCheckFunc(
		resource.TestCheckResourceAttr(n, "id", "5/access"),
		resource.TestCheckResourceAttr(n, "user_id", "5"),
		resource.TestCheckResourceAttr(n, "username", "ci"),
		resource.TestCheckResourceAttr(n, "role", "2"),
		resource.TestCheckResourceAttr(n, "memberships.#", "1"),
		resource.TestCheckResourceAttr(n, "memberships.0.team_id", "3"),
		resource.TestCheckResourceAttr(n, "effective_access.#", "1"),
		resource.TestCheckResourceAttr(n, "effective_access.0.endpoint_name", "prod"),
		resource.TestCheckResourceAttr(n, "effective_access.0.access_location", "group"),
		resource.TestCheckResourceAttr(n, "effective_access.0.team_name", "platform"),
	))
}

// An explicit user_id looks the user up directly instead of resolving the
// caller.
func TestUserAccessDataSource_ExplicitUser(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("GET", "/users/8", RespondJSON(http.StatusOK, map[string]interface{}{"Id": 8, "Username": "alice", "Role": 1}))
	mock.On("GET", "/users/8/memberships", RespondJSON(http.StatusOK, []map[string]interface{}{}))
	mock.On("GET", "/users/8/effective-access", RespondJSON(http.StatusOK, []map[string]interface{}{}))
	n := "data.portainer_user_access.a"
	hUserDSTest(t, mock, `data "portainer_user_access" "a" { user_id = 8 }`, resource.ComposeTestCheckFunc(
		resource.TestCheckResourceAttr(n, "username", "alice"),
		resource.TestCheckResourceAttr(n, "memberships.#", "0"),
		resource.TestCheckResourceAttr(n, "effective_access.#", "0"),
		func(_ *terraform.State) error {
			if mock.FindRequest("GET", "/users/me") != nil {
				return fmt.Errorf("an explicit user_id must not fall back to /users/me")
			}
			return nil
		},
	))
}

func TestUserAccessDataSource_NoIdentifier(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("GET", "/users/me", RespondJSON(http.StatusOK, map[string]interface{}{}))
	hUserDSTestError(t, mock, `data "portainer_user_access" "a" {}`, regexp.MustCompile(`Portainer returned no identifier`))
}

// TestAccUserTeamDataSources reads the admin user, the roles and the
// caller's access on any edition; the activity log and the user's namespaces
// are Business Edition APIs and are read on BE only.
func TestAccUserTeamDataSources(t *testing.T) {
	adminUser := os.Getenv("PORTAINER_ACC_ADMIN_USERNAME")
	if adminUser == "" {
		adminUser = "admin"
	}
	config := accProvider() + `
data "portainer_user" "admin" { username = "` + adminUser + `" }
data "portainer_role" "all" {}
data "portainer_user_access" "me" {}
`
	checks := []resource.TestCheckFunc{
		resource.TestCheckResourceAttr("data.portainer_user.admin", "role", "1"),
		resource.TestCheckResourceAttrSet("data.portainer_role.all", "roles.#"),
		resource.TestCheckResourceAttrSet("data.portainer_user_access.me", "user_id"),
	}
	if accIsBE() {
		config += `
data "portainer_user_activity" "auth" { log_type = "auth" }
data "portainer_user_namespaces" "admin" { user_id = data.portainer_user.admin.id }
`
		checks = append(checks,
			resource.TestCheckResourceAttr("data.portainer_user_activity.auth", "log_type", "auth"),
			resource.TestCheckResourceAttrSet("data.portainer_user_namespaces.admin", "raw"),
		)
	}
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { accPreCheck(t) },
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps:                    []resource.TestStep{{Config: config, Check: resource.ComposeTestCheckFunc(checks...)}},
	})
}
