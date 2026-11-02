package internal

import (
	"net/http"
	"testing"

	"github.com/hashicorp/go-cty/cty"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
)

// Write-only secrets never reach state, so the handlers read them from the
// raw configuration. These tests hand it in through the prior state, which
// GetRawConfig falls back to, and check the secret reaches the payload while
// the state attribute stays empty.

func dataWithRawConfig(r *schema.Resource, attrs map[string]string, raw map[string]cty.Value) *schema.ResourceData {
	attrs["id"] = ""
	d := r.Data(&terraform.InstanceState{Attributes: attrs, RawConfig: cty.ObjectVal(raw)})
	d.SetId("")
	return d
}

func decodeBody(t *testing.T, mock *MockServer, method, path string) map[string]interface{} {
	t.Helper()
	req := mock.FindRequest(method, path)
	if req == nil {
		t.Fatalf("expected %s %s", method, path)
	}
	var body map[string]interface{}
	if err := req.DecodeJSON(&body); err != nil {
		t.Fatalf("decode %s %s: %v", method, path, err)
	}
	return body
}

func TestWriteOnly_CustomTemplateRepositoryPassword(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("GET", "/custom_templates", RespondJSON(http.StatusOK, []interface{}{}))
	mock.On("POST", "/custom_templates/create/repository", RespondJSON(http.StatusOK, map[string]interface{}{"Id": 5}))
	mock.On("GET", "/custom_templates/5", RespondJSON(http.StatusOK, map[string]interface{}{"Id": 5}))

	r := resourceCustomTemplate()
	d := dataWithRawConfig(r, map[string]string{
		"title": "t", "description": "d", "note": "n", "platform": "1", "type": "2",
		"repository_url": "https://git.example.com/app.git", "repository_authentication": "true",
		"repository_username": "bot", "repository_password_wo_version": "1",
		"compose_file_path": "docker-compose.yml", "repository_reference": "refs/heads/main",
	}, map[string]cty.Value{"repository_password_wo": cty.StringVal("git-token-1")})

	if err := rcCreate(r, d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if got := decodeBody(t, mock, "POST", "/custom_templates/create/repository")["repositoryPassword"]; got != "git-token-1" {
		t.Errorf("repositoryPassword: expected the write-only value, got %v", got)
	}
	if d.Get("repository_password").(string) != "" {
		t.Error("repository_password must stay empty in state")
	}
}

func TestWriteOnly_EdgeStackRepositoryPassword(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("GET", "/edge_stacks", RespondJSON(http.StatusOK, []interface{}{}))
	mock.On("POST", "/edge_stacks/create/repository", RespondJSON(http.StatusOK, map[string]interface{}{"Id": 6}))
	mock.On("GET", "/edge_stacks/6", RespondJSON(http.StatusOK, map[string]interface{}{"Id": 6, "Name": "e"}))

	r := resourceEdgeStack()
	d := dataWithRawConfig(r, map[string]string{
		"name": "e", "edge_groups.#": "1", "edge_groups.0": "2",
		"repository_url": "https://git.example.com/edge.git", "git_repository_authentication": "true",
		"repository_username": "bot", "repository_password_wo_version": "1",
	}, map[string]cty.Value{"repository_password_wo": cty.StringVal("git-token-2")})

	if err := rcCreate(r, d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if got := decodeBody(t, mock, "POST", "/edge_stacks/create/repository")["repositoryPassword"]; got != "git-token-2" {
		t.Errorf("repositoryPassword: expected the write-only value, got %v", got)
	}
}

func TestWriteOnly_GitopsSourcePassword(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("POST", "/gitops/sources/git", RespondJSON(http.StatusCreated, map[string]interface{}{"id": 12}))
	mock.On("GET", "/gitops/sources/12", RespondJSON(http.StatusOK, gitopsSourceDetailBody(12)))

	r := resourceGitopsSource()
	d := dataWithRawConfig(r, map[string]string{
		"url": "https://github.com/acme/infra.git", "username": "bot", "password_wo_version": "1",
	}, map[string]cty.Value{"password_wo": cty.StringVal("git-token-3")})

	if err := rcCreate(r, d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	auth, _ := decodeBody(t, mock, "POST", "/gitops/sources/git")["authentication"].(map[string]interface{})
	if auth["password"] != "git-token-3" {
		t.Errorf("authentication.password: expected the write-only value, got %v", auth["password"])
	}
	if d.Get("password").(string) != "" {
		t.Error("password must stay empty in state")
	}
}

func TestWriteOnly_SettingsLDAPAndOAuth(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("PUT", "/settings", RespondJSON(http.StatusOK, map[string]interface{}{}))

	r := resourceSettings()
	d := dataWithRawConfig(r, map[string]string{
		"ldap_settings.#": "1", "ldap_settings.0.reader_dn": "cn=reader", "ldap_settings.0.url": "ldap.example.com:389",
		"ldap_password_wo_version": "1",
		"oauth_settings.#":         "1", "oauth_settings.0.client_id": "portainer",
		"oauth_client_secret_wo_version": "1",
	}, map[string]cty.Value{
		"ldap_password_wo":       cty.StringVal("ldap-secret"),
		"oauth_client_secret_wo": cty.StringVal("oauth-secret"),
	})

	if err := rcCreate(r, d, mock.Client()); err != nil {
		t.Fatalf("apply failed: %v", err)
	}
	body := decodeBody(t, mock, "PUT", "/settings")
	ldap, _ := body["ldapsettings"].(map[string]interface{})
	oauth, _ := body["oauthSettings"].(map[string]interface{})
	if ldap["Password"] != "ldap-secret" {
		t.Errorf("ldapsettings.Password: expected the write-only value, got %v", ldap["Password"])
	}
	if oauth["ClientSecret"] != "oauth-secret" {
		t.Errorf("oauthSettings.ClientSecret: expected the write-only value, got %v", oauth["ClientSecret"])
	}
}

func TestWriteOnly_LDAPSettingsPassword(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("GET", "/settings", RespondJSON(http.StatusOK, map[string]interface{}{}))
	mock.On("PUT", "/settings", RespondJSON(http.StatusOK, map[string]interface{}{}))

	// The state still holds the plaintext password the configuration used
	// before moving to password_wo; the write-only value has to win, and the
	// old one has to leave the state.
	r := resourceLDAPSettings()
	d := dataWithRawConfig(r, map[string]string{
		"reader_dn": "cn=reader", "url": "ldap.example.com:389", "password_wo_version": "1",
		"password": "old-plaintext",
	}, map[string]cty.Value{"password_wo": cty.StringVal("ldap-secret-2")})

	if err := rcCreate(r, d, mock.Client()); err != nil {
		t.Fatalf("apply failed: %v", err)
	}
	ldap, _ := decodeBody(t, mock, "PUT", "/settings")["ldapsettings"].(map[string]interface{})
	if ldap["Password"] != "ldap-secret-2" {
		t.Errorf("ldapsettings.Password: expected the write-only value, got %v", ldap["Password"])
	}
	if got := d.Get("password"); got != "" {
		t.Errorf("password: expected the old plaintext cleared from state, got %v", got)
	}
}

func TestFormatCtyPath(t *testing.T) {
	path := cty.GetAttrPath("ldap_settings").IndexInt(0).GetAttr("password_wo")
	if got := formatCtyPath(path); got != "ldap_settings.0.password_wo" {
		t.Errorf("expected ldap_settings.0.password_wo, got %q", got)
	}
}
