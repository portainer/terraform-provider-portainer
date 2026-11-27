package internal

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/echoprovider"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
	"github.com/hashicorp/terraform-plugin-testing/tfversion"
)

// Ephemeral values never reach the state, so the tests hand them to the echo
// provider (protocol 6, next to the protocol 5 provider under test), whose
// echo.test resource stores what it was given: the only place the value
// may show up.
var testEchoFactories = map[string]func() (tfprotov6.ProviderServer, error){
	"echo": echoprovider.NewProviderServer(),
}

var ephemeralVersionChecks = []tfversion.TerraformVersionCheck{tfversion.SkipBelow(tfversion.Version1_10_0)}

// echoConfig passes the whole ephemeral resource to echo.test.
func echoConfig(address string) string {
	return fmt.Sprintf(`
provider "echo" {
  data = %s
}

resource "echo" "test" {}
`, address)
}

// echoAttr checks one attribute of the echoed ephemeral resource.
func echoAttr(attr string, value knownvalue.Check) statecheck.StateCheck {
	return statecheck.ExpectKnownValue("echo.test", tfjsonpath.New("data").AtMapKey(attr), value)
}

// secretOnlyInEcho fails when the secret is found anywhere in the state but
// in echo.test, or when any portainer_* object made it into the state.
type secretOnlyInEcho struct{ secret *regexp.Regexp }

func (c secretOnlyInEcho) CheckState(_ context.Context, req statecheck.CheckStateRequest, resp *statecheck.CheckStateResponse) {
	if req.State == nil || req.State.Values == nil || req.State.Values.RootModule == nil {
		return
	}
	for _, r := range req.State.Values.RootModule.Resources {
		if strings.HasPrefix(r.Type, "portainer_") {
			resp.Error = fmt.Errorf("%s is in the state", r.Address)
			return
		}
		if r.Address == "echo.test" {
			continue
		}
		raw, _ := json.Marshal(r.AttributeValues)
		if c.secret.Match(raw) {
			resp.Error = fmt.Errorf("the secret leaked into %s", r.Address)
			return
		}
	}
}

// countRequests counts the recorded requests of one method and path.
func countRequests(mock *MockServer, method, path string) int {
	n := 0
	for _, r := range mock.Requests() {
		if r.Method == method && r.Path == path {
			n++
		}
	}
	return n
}

// TestUserAPIKeyEphemeral_OpenClose: every Open (plan and apply each open the
// resource) creates a key with the configured description and password,
// every Close revokes it, and the raw key reaches the consumer only.
func TestUserAPIKeyEphemeral_OpenClose(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	fake := newFakeUsersAPI(mock)
	user := fake.addUser("admin", "pw-1234567890", 1)
	cfg := sessionProviderConfig(mock, "admin", "pw-1234567890") + fmt.Sprintf(`
ephemeral "portainer_user_api_key" "k" {
  user_id     = %d
  description = "ci-run"
  password    = "pw-1234567890"
}
`, user) + echoConfig("ephemeral.portainer_user_api_key.k")

	resource.UnitTest(t, resource.TestCase{
		TerraformVersionChecks:   ephemeralVersionChecks,
		ProtoV5ProviderFactories: testProtoV5Factories,
		ProtoV6ProviderFactories: testEchoFactories,
		Steps: []resource.TestStep{{
			Config: cfg,
			ConfigStateChecks: []statecheck.StateCheck{
				echoAttr("raw_api_key", knownvalue.StringRegexp(regexp.MustCompile(`^ptr_raw_\d+$`))),
				echoAttr("description", knownvalue.StringExact("ci-run")),
				echoAttr("prefix", knownvalue.StringExact("ptr_abc")),
				echoAttr("date_created", knownvalue.Int64Exact(1700000000)),
				secretOnlyInEcho{regexp.MustCompile(`ptr_raw_`)},
			},
		}},
	})

	path := fmt.Sprintf("/users/%d/tokens", user)
	var body map[string]interface{}
	if err := mock.FindRequest("POST", path).DecodeJSON(&body); err != nil || body["description"] != "ci-run" || body["password"] != "pw-1234567890" {
		t.Fatalf("unexpected create payload %v (%v)", body, err)
	}
	created, revoked := countRequests(mock, "POST", path), 0
	for _, r := range mock.Requests() {
		if r.Method == http.MethodDelete && strings.HasPrefix(r.Path, path+"/") {
			revoked++
		}
	}
	if created == 0 || created != revoked {
		t.Fatalf("every opened key must be revoked: %d created, %d revoked", created, revoked)
	}
	if len(fake.keys) != 0 {
		t.Fatalf("keys left behind: %v", fake.keys)
	}
}

// TestUserAPIKeyEphemeral_DefaultDescription: without a description, a unique
// one is generated, so runs never collide on Portainer's uniqueness rule.
func TestUserAPIKeyEphemeral_DefaultDescription(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	fake := newFakeUsersAPI(mock)
	user := fake.addUser("admin", "pw-1234567890", 1)
	resource.UnitTest(t, resource.TestCase{
		TerraformVersionChecks:   ephemeralVersionChecks,
		ProtoV5ProviderFactories: testProtoV5Factories,
		ProtoV6ProviderFactories: testEchoFactories,
		Steps: []resource.TestStep{{
			Config: sessionProviderConfig(mock, "admin", "pw-1234567890") + fmt.Sprintf(`
ephemeral "portainer_user_api_key" "k" {
  user_id  = %d
  password = "pw-1234567890"
}
`, user) + echoConfig("ephemeral.portainer_user_api_key.k"),
			ConfigStateChecks: []statecheck.StateCheck{
				echoAttr("description", knownvalue.StringRegexp(regexp.MustCompile(`^terraform-ephemeral-\d+$`))),
			},
		}},
	})
	if len(fake.keys) != 0 {
		t.Fatalf("keys left behind: %v", fake.keys)
	}
}

// TestUserAPIKeyEphemeral_RequiresSession: like the resource, an API-key
// configured provider is refused before any call is made.
func TestUserAPIKeyEphemeral_RequiresSession(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	resource.UnitTest(t, resource.TestCase{
		TerraformVersionChecks:   ephemeralVersionChecks,
		ProtoV5ProviderFactories: testProtoV5Factories,
		ProtoV6ProviderFactories: testEchoFactories,
		Steps: []resource.TestStep{{
			Config: mockProviderConfig(mock) + `
ephemeral "portainer_user_api_key" "k" {
  user_id  = 1
  password = "pw"
}
` + echoConfig("ephemeral.portainer_user_api_key.k"),
			ExpectError: regexp.MustCompile(`only accepts this call from a\s+session`),
		}},
	})
	if n := countRequests(mock, "POST", "/users/1/tokens"); n != 0 {
		t.Fatalf("expected no create call, got %d", n)
	}
}

// TestScopedKubeconfigEphemeral_OpenClose: Open issues the kubeconfig with
// the resource's payload and Close revokes it.
func TestScopedKubeconfigEphemeral_OpenClose(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("POST", "/kubernetes/1/scoped_kubeconfig", RespondJSON(http.StatusOK, map[string]interface{}{
		"id": 7, "kubeconfig": "apiVersion: v1\nkind: Config\n# kc-secret-token\n", "serviceAccount": "portainer-sa-7",
		"namespaces": []string{"default"}, "expiresAt": 1700003600,
	}))
	mock.On("DELETE", "/kubernetes/1/scoped_kubeconfig/7", RespondString(http.StatusNoContent, "", ""))

	resource.UnitTest(t, resource.TestCase{
		TerraformVersionChecks:   ephemeralVersionChecks,
		ProtoV5ProviderFactories: testProtoV5Factories,
		ProtoV6ProviderFactories: testEchoFactories,
		Steps: []resource.TestStep{{
			Config: mockProviderConfig(mock) + `
ephemeral "portainer_kubernetes_scoped_kubeconfig" "kc" {
  environment_id = 1
  expiry_seconds = 3600
  namespaces     = ["default"]
  description    = "ci"
}
` + echoConfig("ephemeral.portainer_kubernetes_scoped_kubeconfig.kc"),
			ConfigStateChecks: []statecheck.StateCheck{
				echoAttr("kubeconfig", knownvalue.StringRegexp(regexp.MustCompile(`kc-secret-token`))),
				echoAttr("scoped_kubeconfig_id", knownvalue.Int64Exact(7)),
				echoAttr("service_account", knownvalue.StringExact("portainer-sa-7")),
				echoAttr("covered_namespaces", knownvalue.ListExact([]knownvalue.Check{knownvalue.StringExact("default")})),
				echoAttr("expires_at", knownvalue.Int64Exact(1700003600)),
				secretOnlyInEcho{regexp.MustCompile(`kc-secret-token`)},
			},
		}},
	})

	var body map[string]interface{}
	if err := mock.FindRequest("POST", "/kubernetes/1/scoped_kubeconfig").DecodeJSON(&body); err != nil {
		t.Fatal(err)
	}
	if body["expirySeconds"] != float64(3600) || body["includeSecrets"] != false || body["description"] != "ci" || fmt.Sprint(body["namespaces"]) != "[default]" {
		t.Fatalf("unexpected payload %v", body)
	}
	created, revoked := countRequests(mock, "POST", "/kubernetes/1/scoped_kubeconfig"), countRequests(mock, "DELETE", "/kubernetes/1/scoped_kubeconfig/7")
	if created == 0 || created != revoked {
		t.Fatalf("every issued kubeconfig must be revoked: %d issued, %d revoked", created, revoked)
	}
}

// TestScopedKubeconfigEphemeral_CommunityEdition: the 404 of a server without
// the feature is explained as the resource explains it.
func TestScopedKubeconfigEphemeral_CommunityEdition(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("GET", "/system/version", RespondJSON(http.StatusOK, map[string]interface{}{"ServerVersion": "2.39.1", "ServerEdition": "CE"}))
	resource.UnitTest(t, resource.TestCase{
		TerraformVersionChecks:   ephemeralVersionChecks,
		ProtoV5ProviderFactories: testProtoV5Factories,
		ProtoV6ProviderFactories: testEchoFactories,
		Steps: []resource.TestStep{{
			Config: mockProviderConfig(mock) + `
ephemeral "portainer_kubernetes_scoped_kubeconfig" "kc" {
  environment_id = 1
  expiry_seconds = 60
}
` + echoConfig("ephemeral.portainer_kubernetes_scoped_kubeconfig.kc"),
			ExpectError: regexp.MustCompile(`requires Portainer Business Edition`),
		}},
	})
}

// TestKubernetesConfigEphemeral: the same GET as the data source, with the
// environment filters as query parameters.
func TestKubernetesConfigEphemeral(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("GET", "/kubernetes/config", RespondString(http.StatusOK, "text/yaml", "apiVersion: v1\nkind: Config\nusers:\n- user: {token: kcfg-secret}\n"))
	resource.UnitTest(t, resource.TestCase{
		TerraformVersionChecks:   ephemeralVersionChecks,
		ProtoV5ProviderFactories: testProtoV5Factories,
		ProtoV6ProviderFactories: testEchoFactories,
		Steps: []resource.TestStep{{
			Config: mockProviderConfig(mock) + `
ephemeral "portainer_kubernetes_config" "kc" {
  environment_ids         = [1, 2]
  exclude_environment_ids = [3]
}
` + echoConfig("ephemeral.portainer_kubernetes_config.kc"),
			ConfigStateChecks: []statecheck.StateCheck{
				echoAttr("kubeconfig", knownvalue.StringRegexp(regexp.MustCompile(`kcfg-secret`))),
				secretOnlyInEcho{regexp.MustCompile(`kcfg-secret`)},
			},
		}},
	})
	q := mock.FindRequest("GET", "/kubernetes/config").Query
	for _, want := range []string{"ids=1", "ids=2", "excludeIds=3"} {
		if !strings.Contains(q, want) {
			t.Fatalf("query %q lacks %q", q, want)
		}
	}
}

// TestAuthEphemeral_LoginLogout: Open logs in, Close logs out with the issued
// token (not the provider's credentials), so Portainer revokes it.
func TestAuthEphemeral_LoginLogout(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("POST", "/auth", RespondJSON(http.StatusOK, map[string]interface{}{"jwt": "eyJ.ephemeral-jwt"}))
	mock.On("POST", "/auth/logout", RespondString(http.StatusNoContent, "", ""))
	resource.UnitTest(t, resource.TestCase{
		TerraformVersionChecks:   ephemeralVersionChecks,
		ProtoV5ProviderFactories: testProtoV5Factories,
		ProtoV6ProviderFactories: testEchoFactories,
		Steps: []resource.TestStep{{
			Config: mockProviderConfig(mock) + `
ephemeral "portainer_auth" "a" {
  username = "admin"
  password = "s3cret"
}
` + echoConfig("ephemeral.portainer_auth.a"),
			ConfigStateChecks: []statecheck.StateCheck{
				echoAttr("jwt", knownvalue.StringExact("eyJ.ephemeral-jwt")),
				secretOnlyInEcho{regexp.MustCompile(`ephemeral-jwt`)},
			},
		}},
	})

	body := gsbBody(t, mock, "POST", "/auth")
	if body["username"] != "admin" || body["password"] != "s3cret" {
		t.Fatalf("unexpected login payload %v", body)
	}
	logins, logouts := countRequests(mock, "POST", "/auth"), 0
	for _, r := range mock.Requests() {
		if r.Method != http.MethodPost || r.Path != "/auth/logout" {
			continue
		}
		logouts++
		if got := r.Headers.Get("Authorization"); got != "Bearer eyJ.ephemeral-jwt" {
			t.Fatalf("logout sent Authorization %q", got)
		}
		if r.Headers.Get("X-API-Key") != "" {
			t.Fatal("logout must not carry the provider's API key")
		}
	}
	if logins == 0 || logins != logouts {
		t.Fatalf("every login must be logged out: %d logins, %d logouts", logins, logouts)
	}
}

// TestAuthEphemeral_LogoutFailureIsWarning: a failed logout does not fail
// the run, the token expires on its own.
func TestAuthEphemeral_LogoutFailureIsWarning(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("POST", "/auth", RespondJSON(http.StatusOK, map[string]interface{}{"jwt": "eyJ.x"}))
	resource.UnitTest(t, resource.TestCase{
		TerraformVersionChecks:   ephemeralVersionChecks,
		ProtoV5ProviderFactories: testProtoV5Factories,
		ProtoV6ProviderFactories: testEchoFactories,
		Steps: []resource.TestStep{{
			Config: mockProviderConfig(mock) + `
ephemeral "portainer_auth" "a" {
  username = "admin"
  password = "s3cret"
}
` + echoConfig("ephemeral.portainer_auth.a"),
			ConfigStateChecks: []statecheck.StateCheck{echoAttr("jwt", knownvalue.StringExact("eyJ.x"))},
		}},
	})
}

// TestEdgeGenerateKeyEphemeral and TestSSHKeygenEphemeral: the same calls as
// the resources, with the secrets kept out of the state.
func TestEdgeGenerateKeyEphemeral(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("POST", "/endpoints/edge/generate-key", RespondJSON(http.StatusOK, map[string]interface{}{"edgeKey": "edge-secret-key"}))
	resource.UnitTest(t, resource.TestCase{
		TerraformVersionChecks:   ephemeralVersionChecks,
		ProtoV5ProviderFactories: testProtoV5Factories,
		ProtoV6ProviderFactories: testEchoFactories,
		Steps: []resource.TestStep{{
			Config: mockProviderConfig(mock) + `
ephemeral "portainer_endpoints_edge_generate_key" "k" {}
` + echoConfig("ephemeral.portainer_endpoints_edge_generate_key.k"),
			ConfigStateChecks: []statecheck.StateCheck{
				echoAttr("edge_key", knownvalue.StringExact("edge-secret-key")),
				secretOnlyInEcho{regexp.MustCompile(`edge-secret-key`)},
			},
		}},
	})
	if mock.FindRequest("POST", "/endpoints/edge/generate-key") == nil {
		t.Fatal("expected a generate-key call")
	}
}

func TestSSHKeygenEphemeral(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("POST", "/sshkeygen", RespondJSON(http.StatusOK, map[string]interface{}{"public": "ssh-ed25519 AAAA", "private": "-----BEGIN OPENSSH PRIVATE KEY----- ssh-secret"}))
	resource.UnitTest(t, resource.TestCase{
		TerraformVersionChecks:   ephemeralVersionChecks,
		ProtoV5ProviderFactories: testProtoV5Factories,
		ProtoV6ProviderFactories: testEchoFactories,
		Steps: []resource.TestStep{{
			Config: mockProviderConfig(mock) + `
ephemeral "portainer_sshkeygen" "k" {}
` + echoConfig("ephemeral.portainer_sshkeygen.k"),
			ConfigStateChecks: []statecheck.StateCheck{
				echoAttr("public", knownvalue.StringExact("ssh-ed25519 AAAA")),
				echoAttr("private", knownvalue.StringRegexp(regexp.MustCompile(`ssh-secret`))),
				secretOnlyInEcho{regexp.MustCompile(`ssh-secret`)},
			},
		}},
	})
}

// TestAccEphemeral_GeneratedKeys runs the ephemeral resources a Business
// Edition instance supports with an API key: the edge key and the SSH key
// pair reach the consumer and nothing else.
func TestAccEphemeral_GeneratedKeys(t *testing.T) {
	accRequireBE(t, "edge key and SSH key generation")
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { accPreCheck(t) },
		TerraformVersionChecks:   ephemeralVersionChecks,
		ProtoV5ProviderFactories: testProtoV5Factories,
		ProtoV6ProviderFactories: testEchoFactories,
		Steps: []resource.TestStep{{
			Config: accProvider() + `
ephemeral "portainer_endpoints_edge_generate_key" "k" {}
ephemeral "portainer_sshkeygen" "s" {}
` + echoConfig(`{
    edge_key = ephemeral.portainer_endpoints_edge_generate_key.k.edge_key
    private  = ephemeral.portainer_sshkeygen.s.private
  }`),
			ConfigStateChecks: []statecheck.StateCheck{
				echoAttr("edge_key", knownvalue.StringRegexp(regexp.MustCompile(`^\S{20,}$`))),
				echoAttr("private", knownvalue.StringRegexp(regexp.MustCompile(`PRIVATE KEY`))),
				secretOnlyInEcho{regexp.MustCompile(`PRIVATE KEY`)},
			},
		}},
	})
}
