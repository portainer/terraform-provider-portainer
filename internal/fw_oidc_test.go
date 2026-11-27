package internal

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// fakeOIDC is Portainer 3.1's /oidc_clients API for client 1 and secrets
// s1..s4: POST creates, PUT merges the fields it gets (absent ones are left
// unchanged), and secrets are added and regenerated with fresh IDs.
type fakeOIDC struct {
	mu         sync.Mutex
	client     map[string]any // nil until created
	nextSecret int
}

func newFakeOIDC(m *MockServer) *fakeOIDC {
	f := &fakeOIDC{nextSecret: 1}
	m.On("POST", "/oidc_clients", func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		f.mu.Lock()
		c := map[string]any{"id": 1, "clientId": "cid-1", "enabled": true, "permission": "none", "secrets": []any{}}
		for k, v := range body {
			if k != "secretExpiresAt" {
				c[k] = v
			}
		}
		exp, _ := body["secretExpiresAt"].(float64)
		c["secrets"] = []any{f.newSecretLocked("", int64(exp))}
		f.client = c
		out := map[string]any{"client": c, "secret": "first-secret"}
		f.mu.Unlock()
		RespondJSON(http.StatusOK, out)(w, r)
	})
	m.On("GET", "/oidc_clients", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		out := []any{}
		if f.client != nil {
			out = append(out, f.client)
		}
		RespondJSON(http.StatusOK, out)(w, r)
	})
	m.On("GET", "/oidc_clients/1", f.withClient(func(w http.ResponseWriter, r *http.Request) {
		RespondJSON(http.StatusOK, f.client)(w, r)
	}))
	m.On("PUT", "/oidc_clients/1", f.withClient(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		for k, v := range body {
			f.client[k] = v
		}
		RespondJSON(http.StatusOK, f.client)(w, r)
	}))
	m.On("DELETE", "/oidc_clients/1", f.withClient(func(w http.ResponseWriter, _ *http.Request) {
		f.client = nil
		w.WriteHeader(http.StatusNoContent)
	}))
	m.On("POST", "/oidc_clients/1/secrets", f.withClient(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Description string `json:"description"`
			ExpiresAt   int64  `json:"expiresAt"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		s := f.newSecretLocked(body.Description, body.ExpiresAt)
		f.client["secrets"] = append(f.client["secrets"].([]any), s)
		RespondJSON(http.StatusOK, map[string]any{"secret": s, "value": fmt.Sprintf("value-%s", s["id"])})(w, r)
	}))
	for i := 1; i <= 4; i++ {
		id := fmt.Sprintf("s%d", i)
		m.On("DELETE", "/oidc_clients/1/secrets/"+id, f.withClient(func(w http.ResponseWriter, _ *http.Request) {
			f.dropSecretLocked(id)
			w.WriteHeader(http.StatusNoContent)
		}))
		m.On("POST", "/oidc_clients/1/secrets/"+id+"/regenerate", f.withClient(func(w http.ResponseWriter, r *http.Request) {
			var body struct {
				OldSecretExpiresAt int64 `json:"oldSecretExpiresAt"`
			}
			_ = json.NewDecoder(r.Body).Decode(&body)
			old := f.secretLocked(id)
			if old == nil {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			if body.OldSecretExpiresAt == 0 {
				f.dropSecretLocked(id)
			}
			// Same description and lifetime: a later expiry.
			exp := old["expiresAt"].(int64)
			if exp != 0 {
				exp += 1000
			}
			s := f.newSecretLocked(old["description"].(string), exp)
			f.client["secrets"] = append(f.client["secrets"].([]any), s)
			RespondJSON(http.StatusOK, map[string]any{"secret": s, "value": fmt.Sprintf("value-%s", s["id"])})(w, r)
		}))
	}
	return f
}

// withClient runs h under the lock, answering 404 while there is no client.
func (f *fakeOIDC) withClient(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		if f.client == nil {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		h(w, r)
	}
}

func (f *fakeOIDC) newSecretLocked(desc string, expiresAt int64) map[string]any {
	id := fmt.Sprintf("s%d", f.nextSecret)
	f.nextSecret++
	return map[string]any{"id": id, "description": desc, "hint": "h" + id, "createdAt": int64(100 * f.nextSecret), "expiresAt": expiresAt, "digest": "d"}
}

func (f *fakeOIDC) secretLocked(id string) map[string]any {
	for _, s := range f.client["secrets"].([]any) {
		if s.(map[string]any)["id"] == id {
			return s.(map[string]any)
		}
	}
	return nil
}

func (f *fakeOIDC) dropSecretLocked(id string) {
	var keep []any
	for _, s := range f.client["secrets"].([]any) {
		if s.(map[string]any)["id"] != id {
			keep = append(keep, s)
		}
	}
	f.client["secrets"] = keep
}

func (f *fakeOIDC) set(k string, v any) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.client[k] = v
}

func oidcRequestBody(t *testing.T, mock *MockServer, method, path string, n int) map[string]any {
	t.Helper()
	var seen int
	for _, r := range mock.Requests() {
		if r.Method == method && r.Path == path {
			seen++
			if seen == n {
				var body map[string]any
				if err := r.DecodeJSON(&body); err != nil {
					t.Fatalf("%s %s: %v", method, path, err)
				}
				return body
			}
		}
	}
	t.Fatalf("expected request %d of %s %s, got %d", n, method, path, seen)
	return nil
}

func oidcCount(mock *MockServer, method, path string) int {
	n := 0
	for _, r := range mock.Requests() {
		if r.Method == method && r.Path == path {
			n++
		}
	}
	return n
}

const oidcClientFull = `
resource "portainer_oidc_client" "c" {
  name                      = "Grafana"
  description               = "Dashboards"
  redirect_uris             = ["https://grafana.example.com/login/generic_oauth"]
  post_logout_redirect_uris = ["https://grafana.example.com/"]
  homepage_url              = "https://grafana.example.com"
  permission                = "read-only"
  groups_claim              = true
  show_in_launcher          = true
  enabled                   = false
  secret_expires_at         = 1900000000
  user_access_policies      = { "3" = 1 }
  team_access_policies      = { "2" = 1 }
}`

// TestOIDCClientResource_Lifecycle: the create payload carries the
// configuration, a client configured off is switched off by a PUT right
// after the create, the first secret is kept, the next plan is empty, an
// update clears what the configuration drops, and the client imports.
func TestOIDCClientResource_Lifecycle(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	fake := newFakeOIDC(mock)
	addr := "portainer_oidc_client.c"
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{
				Config: mockProviderConfig(mock) + oidcClientFull,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(addr, "id", "1"),
					resource.TestCheckResourceAttr(addr, "client_id", "cid-1"),
					resource.TestCheckResourceAttr(addr, "client_secret", "first-secret"),
					resource.TestCheckResourceAttr(addr, "client_secret_id", "s1"),
					resource.TestCheckResourceAttr(addr, "enabled", "false"),
					resource.TestCheckResourceAttr(addr, "all_users", "false"),
					resource.TestCheckResourceAttr(addr, "user_access_policies.3", "1"),
					func(*terraform.State) error {
						p := oidcRequestBody(t, mock, "POST", "/oidc_clients", 1)
						if _, ok := p["enabled"]; ok {
							return fmt.Errorf("create payload: enabled is not a create field, got %v", p)
						}
						if p["name"] != "Grafana" || p["permission"] != "read-only" || p["secretExpiresAt"] != float64(1900000000) || p["groupsClaim"] != true {
							return fmt.Errorf("create payload: got %v", p)
						}
						if fmt.Sprint(p["userAccessPolicies"]) != "map[3:map[RoleId:1]]" || fmt.Sprint(p["redirectUris"]) != "[https://grafana.example.com/login/generic_oauth]" {
							return fmt.Errorf("create payload: policies/redirects %v", p)
						}
						u := oidcRequestBody(t, mock, "PUT", "/oidc_clients/1", 1)
						if u["enabled"] != false || u["name"] != "Grafana" {
							return fmt.Errorf("switch-off update: got %v", u)
						}
						return nil
					},
				),
			},
			{Config: mockProviderConfig(mock) + oidcClientFull, PlanOnly: true},
			{
				// Drift in Portainer shows up as a change.
				PreConfig:          func() { fake.set("name", "Changed") },
				Config:             mockProviderConfig(mock) + oidcClientFull,
				PlanOnly:           true,
				ExpectNonEmptyPlan: true,
			},
			{
				Config: mockProviderConfig(mock) + `
resource "portainer_oidc_client" "c" {
  name              = "Grafana"
  redirect_uris     = ["https://grafana.example.com/login/generic_oauth"]
  secret_expires_at = 1900000000
}`,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction(addr, plancheck.ResourceActionUpdate)},
				},
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(addr, "client_secret", "first-secret"),
					resource.TestCheckResourceAttr(addr, "permission", "none"),
					resource.TestCheckResourceAttr(addr, "enabled", "true"),
					resource.TestCheckNoResourceAttr(addr, "description"),
					resource.TestCheckNoResourceAttr(addr, "post_logout_redirect_uris"),
					resource.TestCheckNoResourceAttr(addr, "user_access_policies"),
					func(*terraform.State) error {
						u := oidcRequestBody(t, mock, "PUT", "/oidc_clients/1", oidcCount(mock, "PUT", "/oidc_clients/1"))
						if u["description"] != "" || u["homepageUrl"] != "" || u["enabled"] != true || u["permission"] != "none" {
							return fmt.Errorf("update payload: expected cleared values, got %v", u)
						}
						if fmt.Sprint(u["userAccessPolicies"]) != "map[]" || fmt.Sprint(u["postLogoutRedirectUris"]) != "[]" {
							return fmt.Errorf("update payload: expected emptied lists and policies, got %v", u)
						}
						return nil
					},
				),
			},
			{
				ResourceName:            addr,
				ImportState:             true,
				ImportStateId:           "1",
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"client_secret", "client_secret_id", "secret_expires_at"},
			},
		},
	})
}

// TestOIDCClientResource_OlderServerNamesRequirement: Portainer 3.0 has no
// OIDC provider; its 404 says what the client needs.
func TestOIDCClientResource_OlderServerNamesRequirement(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("POST", "/oidc_clients", RespondString(http.StatusNotFound, "text/plain", "404 page not found"))
	mock.On("GET", "/system/version", RespondJSON(http.StatusOK, map[string]any{"ServerVersion": "3.0.1", "ServerEdition": "EE"}))
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{{
			Config: mockProviderConfig(mock) + `
resource "portainer_oidc_client" "c" {
  name          = "Grafana"
  redirect_uris = ["https://grafana.example.com/cb"]
}`,
			ExpectError: regexp.MustCompile(`portainer_oidc_client requires Portainer\s+Business\s+Edition\s+3.1\s+or\s+later`),
		}},
	})
}

var fwOIDCClientCase = fwIdentityCase{
	typ:   "portainer_oidc_client",
	setup: func(m *MockServer) { newFakeOIDC(m) },
	config: `
resource "portainer_oidc_client" "r" {
  name          = "Grafana"
  redirect_uris = ["https://grafana.example.com/cb"]
}`,
	display: "Grafana",
	values:  map[string]knownvalue.Check{"id": knownvalue.StringExact("1"), "name": knownvalue.StringExact("Grafana"), "client_id": knownvalue.StringExact("cid-1")},
	filter:  `name = "Grafana"`,
	miss:    `name = "Other"`,
}

func TestOIDCClientResource_Identity(t *testing.T) {
	t.Parallel()
	testFWIdentity(t, fwOIDCClientCase)
}

func TestOIDCClientResource_List(t *testing.T) {
	t.Parallel()
	testFWList(t, fwOIDCClientCase)
}

func oidcSecretConfig(m *MockServer, extra string) string {
	return mockProviderConfig(m) + `
resource "portainer_oidc_client" "c" {
  name          = "Grafana"
  redirect_uris = ["https://grafana.example.com/cb"]
}

resource "portainer_oidc_client_secret" "s" {
  client_id   = tonumber(portainer_oidc_client.c.id)
  description = "rotation"
  expires_at  = 1900000000
  ` + extra + `
}`
}

// TestOIDCClientSecretResource_Lifecycle: a secret is added with its value
// kept in state; a new regenerate_version regenerates it in place, retiring
// the old one after the grace period; it imports by "<client>:<secret>".
func TestOIDCClientSecretResource_Lifecycle(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	newFakeOIDC(mock)
	addr := "portainer_oidc_client_secret.s"
	start := time.Now().Unix()
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{
				Config: oidcSecretConfig(mock, `regenerate_version = 1`),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(addr, "id", "1:s2"),
					resource.TestCheckResourceAttr(addr, "secret_id", "s2"),
					resource.TestCheckResourceAttr(addr, "value", "value-s2"),
					resource.TestCheckResourceAttr(addr, "hint", "hs2"),
					resource.TestCheckResourceAttr(addr, "effective_expires_at", "1900000000"),
					func(*terraform.State) error {
						p := oidcRequestBody(t, mock, "POST", "/oidc_clients/1/secrets", 1)
						if p["description"] != "rotation" || p["expiresAt"] != float64(1900000000) {
							return fmt.Errorf("create payload: got %v", p)
						}
						return nil
					},
				),
			},
			{Config: oidcSecretConfig(mock, `regenerate_version = 1`), PlanOnly: true},
			{
				Config: oidcSecretConfig(mock, `regenerate_version = 2
  old_secret_grace_period = "1h"`),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction(addr, plancheck.ResourceActionUpdate)},
				},
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(addr, "id", "1:s3"),
					resource.TestCheckResourceAttr(addr, "value", "value-s3"),
					resource.TestCheckResourceAttr(addr, "expires_at", "1900000000"),
					resource.TestCheckResourceAttr(addr, "effective_expires_at", "1900001000"),
					func(*terraform.State) error {
						p := oidcRequestBody(t, mock, "POST", "/oidc_clients/1/secrets/s2/regenerate", 1)
						old, _ := p["oldSecretExpiresAt"].(float64)
						if int64(old) < start+3600 || int64(old) > time.Now().Unix()+3600 {
							return fmt.Errorf("regenerate payload: oldSecretExpiresAt %v is not an hour from now", p)
						}
						return nil
					},
				),
			},
			{
				Config: oidcSecretConfig(mock, `regenerate_version = 2
  old_secret_grace_period = "1h"`),
				PlanOnly: true,
			},
			{
				ResourceName:            addr,
				ImportState:             true,
				ImportStateId:           "1:s3",
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"value", "regenerate_version", "old_secret_grace_period", "expires_at"},
			},
			{
				ResourceName:  addr,
				ImportState:   true,
				ImportStateId: "s3",
				ExpectError:   regexp.MustCompile(`expected "<client id>:<secret id>"`),
			},
		},
		CheckDestroy: func(*terraform.State) error {
			if mock.FindRequest("DELETE", "/oidc_clients/1/secrets/s3") == nil {
				return fmt.Errorf("expected the regenerated secret to be deleted")
			}
			return nil
		},
	})
}

// TestOIDCClientSecretResource_GoneIsRemoved: a secret Portainer no longer
// has (deleted, or expired) leaves the state, so the next plan adds it again.
func TestOIDCClientSecretResource_GoneIsRemoved(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	fake := newFakeOIDC(mock)
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{Config: oidcSecretConfig(mock, "")},
			{
				PreConfig: func() {
					fake.mu.Lock()
					fake.dropSecretLocked("s2")
					fake.mu.Unlock()
				},
				Config:             oidcSecretConfig(mock, ""),
				PlanOnly:           true,
				ExpectNonEmptyPlan: true,
			},
		},
	})
}

// TestOIDCOperationResources: each one-shot resource sends its request once
// and again only when its triggers change.
func TestOIDCOperationResources(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("POST", "/oidc_keys/rotate", RespondString(http.StatusNoContent, "", ""))
	mock.On("DELETE", "/oidc_clients/4/sessions", RespondString(http.StatusNoContent, "", ""))
	mock.On("DELETE", "/oidc_sessions", RespondString(http.StatusNoContent, "", ""))
	config := func(rev string) string {
		return mockProviderConfig(mock) + fmt.Sprintf(`
resource "portainer_oidc_keys_rotate" "k" {
  triggers = { rev = %[1]q }
}

resource "portainer_oidc_client_sessions_end" "c" {
  client_id = 4
  user_id   = 7
}

resource "portainer_oidc_client_sessions_end" "all" {
  client_id = 4
}

resource "portainer_oidc_user_sessions_end" "u" {
  user_id = 7
}`, rev)
	}
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{
				Config: config("1"),
				Check: func(*terraform.State) error {
					var queries []string
					for _, r := range mock.Requests() {
						if r.Path == "/oidc_clients/4/sessions" {
							queries = append(queries, r.Query)
						}
					}
					if len(queries) != 2 || !strings.Contains(strings.Join(queries, ","), "userId=7") || (queries[0] != "" && queries[1] != "") {
						return fmt.Errorf("client sessions: expected one request with userId=7 and one without, got %q", queries)
					}
					if r := mock.FindRequest("DELETE", "/oidc_sessions"); r == nil || r.Query != "userId=7" {
						return fmt.Errorf("user sessions: expected DELETE /oidc_sessions?userId=7")
					}
					return nil
				},
			},
			{Config: config("1"), PlanOnly: true},
			{
				Config: config("2"),
				Check: func(*terraform.State) error {
					if n := oidcCount(mock, "POST", "/oidc_keys/rotate"); n != 2 {
						return fmt.Errorf("expected a second rotation, got %d", n)
					}
					if n := oidcCount(mock, "DELETE", "/oidc_sessions"); n != 1 {
						return fmt.Errorf("expected the user sessions to end once, got %d", n)
					}
					return nil
				},
			},
		},
	})
}

func TestOIDCKeysRotateAction(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("POST", "/oidc_keys/rotate", RespondString(http.StatusNoContent, "", ""))
	testAction(t, actionConfig(mock, "portainer_oidc_keys_rotate", ``),
		expectRequests(mock, "POST /oidc_keys/rotate"))
}

func TestOIDCClientSessionsEndAction(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("DELETE", "/oidc_clients/4/sessions", RespondString(http.StatusNoContent, "", ""))
	testAction(t, actionConfig(mock, "portainer_oidc_client_sessions_end", `client_id = 4
    user_id = 7`),
		resource.ComposeTestCheckFunc(
			expectRequests(mock, "DELETE /oidc_clients/4/sessions"),
			expectQuery(mock, "DELETE /oidc_clients/4/sessions", "userId=7"),
		))
}

func TestOIDCUserSessionsEndAction(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("DELETE", "/oidc_sessions", RespondString(http.StatusNoContent, "", ""))
	testAction(t, actionConfig(mock, "portainer_oidc_user_sessions_end", `user_id = 7`),
		resource.ComposeTestCheckFunc(
			expectRequests(mock, "DELETE /oidc_sessions"),
			expectQuery(mock, "DELETE /oidc_sessions", "userId=7"),
		))
}

// TestOIDCDataSources: clients (filtered by name, secrets without values),
// a client's sessions and the launcher.
func TestOIDCDataSources(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("GET", "/oidc_clients", RespondJSON(http.StatusOK, []any{
		map[string]any{"id": 2, "clientId": "cid-2", "name": "Vault", "permission": "none", "enabled": true},
		map[string]any{
			"id": 1, "clientId": "cid-1", "name": "Grafana", "permission": "read-only", "enabled": true, "showInLauncher": true,
			"redirectUris":       []string{"https://grafana.example.com/cb"},
			"userAccessPolicies": map[string]any{"3": map[string]any{"RoleId": 1}},
			"secrets":            []any{map[string]any{"id": "s1", "hint": "abcd", "createdAt": 100, "expiresAt": 0, "digest": "x"}},
		},
	}))
	mock.On("GET", "/oidc_clients/1/sessions", RespondJSON(http.StatusOK, []any{
		map[string]any{"id": 9, "userId": 3, "username": "alice", "createdAt": 100, "expiresAt": 200, "lastUsedAt": 150, "refreshable": true},
	}))
	mock.On("GET", "/oidc_launcher", RespondJSON(http.StatusOK, []any{
		map[string]any{"name": "Grafana", "description": "Dashboards", "homepageUrl": "https://grafana.example.com"},
	}))
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{{
			Config: mockProviderConfig(mock) + `
data "portainer_oidc_clients" "all" {}

data "portainer_oidc_clients" "grafana" {
  name = "Grafana"
}

data "portainer_oidc_client_sessions" "s" {
  client_id = 1
}

data "portainer_oidc_launcher" "l" {}
`,
			Check: resource.ComposeTestCheckFunc(
				resource.TestCheckResourceAttr("data.portainer_oidc_clients.all", "clients.#", "2"),
				resource.TestCheckResourceAttr("data.portainer_oidc_clients.all", "clients.0.id", "1"),
				resource.TestCheckResourceAttr("data.portainer_oidc_clients.grafana", "clients.#", "1"),
				resource.TestCheckResourceAttr("data.portainer_oidc_clients.grafana", "clients.0.client_id", "cid-1"),
				resource.TestCheckResourceAttr("data.portainer_oidc_clients.grafana", "clients.0.user_access_policies.3", "1"),
				resource.TestCheckResourceAttr("data.portainer_oidc_clients.grafana", "clients.0.secrets.0.hint", "abcd"),
				resource.TestCheckResourceAttr("data.portainer_oidc_client_sessions.s", "sessions.0.username", "alice"),
				resource.TestCheckResourceAttr("data.portainer_oidc_client_sessions.s", "sessions.0.refreshable", "true"),
				resource.TestCheckResourceAttr("data.portainer_oidc_launcher.l", "entries.0.homepage_url", "https://grafana.example.com"),
			),
		}},
	})
}

// TestSettingsResource_OIDCProvider: the oidc_provider block is sent as the
// top-level OIDCProvider* fields of PUT /settings, read back from
// OIDCProviderSettings, and left out when not configured.
func TestSettingsResource_OIDCProvider(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	fake := newFakePortainerSettings(mock, portainerDefaultSettings())
	config := mockProviderConfig(mock) + `
resource "portainer_settings" "s" {
  oidc_provider {
    public_url            = "https://portainer.example.com"
    access_token_lifetime = "10m"
  }
}`
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{
				Config: mockProviderConfig(mock) + `
resource "portainer_settings" "s" {
  oidc_provider {
    access_token_lifetime = "ten minutes"
  }
}`,
				ExpectError: regexp.MustCompile(`Invalid duration`),
			},
			{
				Config: config,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("portainer_settings.s", "oidc_provider.0.public_url", "https://portainer.example.com"),
					resource.TestCheckResourceAttr("portainer_settings.s", "oidc_provider.0.access_token_lifetime", "10m"),
					resource.TestCheckResourceAttr("portainer_settings.s", "oidc_provider.0.refresh_token_idle_lifetime", ""),
					func(*terraform.State) error {
						p := gsbBody(t, mock, "PUT", "/settings")
						if p["OIDCProviderPublicURL"] != "https://portainer.example.com" || p["OIDCProviderAccessTokenLifetime"] != "10m" {
							return fmt.Errorf("payload: got %v", p)
						}
						for _, k := range []string{"OIDCProviderRefreshTokenIdleLifetime", "OIDCProviderRefreshTokenAbsoluteLifetime"} {
							if _, ok := p[k]; ok {
								return fmt.Errorf("payload.%s: expected it left out when not configured", k)
							}
						}
						return nil
					},
				),
			},
			{Config: config, PlanOnly: true},
			{
				PreConfig: func() {
					fake.set("oidcprovidersettings", map[string]any{"PublicURL": "https://other.example.com", "AccessTokenLifetime": "10m"})
				},
				Config:             config,
				PlanOnly:           true,
				ExpectNonEmptyPlan: true,
			},
			{
				Config: mockProviderConfig(mock) + `resource "portainer_settings" "s" { logo_url = "https://example.com/logo.png" }`,
				Check: func(*terraform.State) error {
					p := gsbBody(t, mock, "PUT", "/settings")
					for k := range p {
						if strings.HasPrefix(k, "OIDCProvider") {
							return fmt.Errorf("payload.%s: expected it left out when not configured", k)
						}
					}
					return nil
				},
			},
			{Config: mockProviderConfig(mock) + `resource "portainer_settings" "s" { logo_url = "https://example.com/logo.png" }`, PlanOnly: true},
		},
	})
}

// TestAccOIDCClient creates a client and a second secret on a live Business
// Edition 3.1 server.
func TestAccOIDCClient(t *testing.T) {
	accPreCheck(t)
	accRequireBE(t, "the OIDC provider (Business Edition 3.1)")
	accRequireVersion(t, "3.1.0", "the OIDC provider")
	resource.Test(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{{
			Config: accProvider() + `
resource "portainer_oidc_client" "c" {
  name          = "tf-acc-oidc"
  redirect_uris = ["https://tf-acc.example.com/cb"]
}

resource "portainer_oidc_client_secret" "s" {
  client_id   = tonumber(portainer_oidc_client.c.id)
  description = "tf-acc"
}`,
			Check: resource.ComposeTestCheckFunc(
				resource.TestCheckResourceAttrSet("portainer_oidc_client.c", "client_id"),
				resource.TestCheckResourceAttrSet("portainer_oidc_client.c", "client_secret"),
				resource.TestCheckResourceAttrSet("portainer_oidc_client_secret.s", "value"),
			),
		}},
	})
}
