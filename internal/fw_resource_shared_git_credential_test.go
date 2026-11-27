package internal

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"sync"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// fakeGitCredentials is a stateful stand-in for Portainer's git credential
// API under one base path (/cloud/gitcredentials or /users/{id}/gitcredentials).
type fakeGitCredentials struct {
	mu        sync.Mutex
	nextID    int
	userID    int
	creds     map[int]gitCredentialAPI
	passwords map[int]string
}

func newFakeGitCredentials(m *MockServer, base string, userID int, maxID int) *fakeGitCredentials {
	f := &fakeGitCredentials{userID: userID, creds: map[int]gitCredentialAPI{}, passwords: map[int]string{}}
	type body struct {
		Name              string `json:"name"`
		Username          string `json:"username"`
		Password          string `json:"password"`
		AuthorizationType int    `json:"authorizationType"`
	}
	m.On("GET", base, func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		list := []gitCredentialAPI{}
		for i := 1; i <= f.nextID; i++ {
			if c, ok := f.creds[i]; ok {
				list = append(list, c)
			}
		}
		RespondJSON(http.StatusOK, list)(w, r)
	})
	m.On("POST", base, func(w http.ResponseWriter, r *http.Request) {
		var b body
		_ = json.NewDecoder(r.Body).Decode(&b)
		f.mu.Lock()
		defer f.mu.Unlock()
		f.nextID++
		c := gitCredentialAPI{ID: f.nextID, Name: b.Name, Username: b.Username, AuthorizationType: b.AuthorizationType, UserID: f.userID}
		f.creds[c.ID] = c
		f.passwords[c.ID] = b.Password
		RespondJSON(http.StatusOK, map[string]interface{}{"gitCredential": c})(w, r)
	})
	for id := 1; id <= maxID; id++ {
		id := id
		p := fmt.Sprintf("%s/%d", base, id)
		m.On("GET", p, func(w http.ResponseWriter, r *http.Request) {
			f.mu.Lock()
			defer f.mu.Unlock()
			c, ok := f.creds[id]
			if !ok {
				RespondString(http.StatusNotFound, "application/json", `{"message":"not found"}`)(w, r)
				return
			}
			RespondJSON(http.StatusOK, c)(w, r)
		})
		m.On("PUT", p, func(w http.ResponseWriter, r *http.Request) {
			var b body
			_ = json.NewDecoder(r.Body).Decode(&b)
			f.mu.Lock()
			defer f.mu.Unlock()
			c := f.creds[id]
			c.Name, c.Username, c.AuthorizationType = b.Name, b.Username, b.AuthorizationType
			f.creds[id] = c
			f.passwords[id] = b.Password
			w.WriteHeader(http.StatusNoContent)
		})
		m.On("DELETE", p, func(w http.ResponseWriter, r *http.Request) {
			f.mu.Lock()
			defer f.mu.Unlock()
			if _, ok := f.creds[id]; !ok {
				RespondString(http.StatusNotFound, "application/json", `{"message":"not found"}`)(w, r)
				return
			}
			delete(f.creds, id)
			w.WriteHeader(http.StatusNoContent)
		})
	}
	return f
}

func (f *fakeGitCredentials) password(id int) string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.passwords[id]
}

// TestSharedGitCredentialResource_Lifecycle: create (unwrapping the
// "gitCredential" envelope, issue #120), update with PUT, import, destroy.
func TestSharedGitCredentialResource_Lifecycle(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	fake := newFakeGitCredentials(mock, "/cloud/gitcredentials", 0, 3)
	cfg := func(username, password string) string {
		return mockProviderConfig(mock) + fmt.Sprintf(`
resource "portainer_shared_git_credential" "g" {
  name               = "github_test"
  username           = %q
  password           = %q
  authorization_type = 1
}
`, username, password)
	}

	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		CheckDestroy: func(*terraform.State) error {
			if mock.FindRequest("DELETE", "/cloud/gitcredentials/1") == nil {
				return fmt.Errorf("expected DELETE /cloud/gitcredentials/1")
			}
			return nil
		},
		Steps: []resource.TestStep{
			{
				Config: cfg("testtoken", "some_token"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("portainer_shared_git_credential.g", "id", "1"),
					resource.TestCheckResourceAttr("portainer_shared_git_credential.g", "user_id", "0"),
					resource.TestCheckResourceAttr("portainer_shared_git_credential.g", "authorization_type", "1"),
					func(*terraform.State) error {
						var payload map[string]interface{}
						post := mock.FindRequest("POST", "/cloud/gitcredentials")
						if post == nil || post.DecodeJSON(&payload) != nil {
							return fmt.Errorf("expected a JSON POST")
						}
						if payload["name"] != "github_test" || payload["username"] != "testtoken" ||
							payload["password"] != "some_token" || payload["authorizationType"] != float64(1) {
							return fmt.Errorf("unexpected payload %v", payload)
						}
						return nil
					},
				),
			},
			{
				Config: cfg("robot", "other_token"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction("portainer_shared_git_credential.g", plancheck.ResourceActionUpdate)},
				},
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("portainer_shared_git_credential.g", "username", "robot"),
					func(*terraform.State) error {
						if fake.password(1) != "other_token" {
							return fmt.Errorf("the new password was not sent with PUT")
						}
						return nil
					},
				),
			},
			{
				ResourceName:            "portainer_shared_git_credential.g",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"password"},
			},
		},
	})
}

func TestSharedGitCredentialResource_CreateHTTPError(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("POST", "/cloud/gitcredentials", RespondString(http.StatusBadRequest, "application/json", `{"message":"invalid authorization type"}`))
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{{
			Config:      mockProviderConfig(mock) + `resource "portainer_shared_git_credential" "g" {` + "\n" + `name = "bad"` + "\n" + `username = "x"` + "\n" + `password = "y"` + "\n}",
			ExpectError: regexp.MustCompile(`failed to create shared git credential`),
		}},
	})
}

// TestSharedGitCredentialResource_URLPatternRequired: Business Edition 3.1
// rejects a credential without urlPattern; the error names url_pattern.
func TestSharedGitCredentialResource_URLPatternRequired(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("POST", "/cloud/gitcredentials", RespondString(http.StatusBadRequest, "application/json",
		`{"message":"Invalid request payload","details":"At least one URL pattern is required"}`))
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{{
			Config:      mockProviderConfig(mock) + `resource "portainer_shared_git_credential" "g" {` + "\n" + `name = "bad"` + "\n" + `username = "x"` + "\n" + `password = "y"` + "\n}",
			ExpectError: regexp.MustCompile(`Business\s+Edition\s+3\.1\s+and\s+later\s+require\s+url_pattern[\s\S]*set\s+url_pattern`),
		}},
	})
}

func TestSharedGitCredentialResource_Validation(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{
				Config:      mockProviderConfig(mock) + `resource "portainer_shared_git_credential" "g" {` + "\n" + `name = ""` + "\n" + `username = "x"` + "\n" + `password = "y"` + "\n}",
				ExpectError: regexp.MustCompile(`at least 1`),
			},
			{
				Config:      mockProviderConfig(mock) + `resource "portainer_shared_git_credential" "g" {` + "\n" + `name = "n"` + "\n" + `username = "x"` + "\n" + `password = "y"` + "\n" + `authorization_type = 2` + "\n}",
				ExpectError: regexp.MustCompile(`between 0 and 1`),
			},
		},
	})
}

// TestSharedGitCredentialResource_GoneOutsideTerraform: a 404 on read plans
// the credential for creation again.
func TestSharedGitCredentialResource_GoneOutsideTerraform(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	fake := newFakeGitCredentials(mock, "/cloud/gitcredentials", 0, 2)
	config := mockProviderConfig(mock) + `resource "portainer_shared_git_credential" "g" {` + "\n" + `name = "n"` + "\n" + `username = "x"` + "\n" + `password = "y"` + "\n}"
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{Config: config},
			{
				PreConfig: func() {
					fake.mu.Lock()
					delete(fake.creds, 1)
					fake.mu.Unlock()
				},
				Config:             config,
				PlanOnly:           true,
				ExpectNonEmptyPlan: true,
			},
		},
	})
}

func TestSharedGitCredentialResource_UpgradeFromV300(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	newFakeGitCredentials(mock, "/cloud/gitcredentials", 3, 3)
	config := mockProviderConfig(mock) + `
resource "portainer_shared_git_credential" "basic" {
  name     = "basic"
  username = "u"
  password = "p"
}
resource "portainer_shared_git_credential" "token" {
  name               = "token"
  username           = "u"
  password           = "p"
  authorization_type = 1
}
`
	resource.Test(t, resource.TestCase{
		IsUnitTest: true,
		Steps: []resource.TestStep{
			{ExternalProviders: previousRelease, Config: config},
			{
				ProtoV5ProviderFactories: testProtoV5Factories,
				Config:                   config,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
		},
	})
}

// TestAccSharedGitCredentialResource needs Business Edition (shared git
// credentials are a BE feature).
func TestAccSharedGitCredentialResource(t *testing.T) {
	if os.Getenv("PORTAINER_ACC_BE") != "1" {
		t.Skip("shared git credentials need Portainer Business Edition; set PORTAINER_ACC_BE=1")
	}
	config := accProvider() + fmt.Sprintf(`
resource "portainer_shared_git_credential" "g" {
  name        = %q
  username    = "robot"
  password    = "token"
  url_pattern = "github.com/acc-test/*" # required by Business Edition 3.1 and later
}
`, acctest.RandomWithPrefix("acc-git"))
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { accPreCheck(t) },
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{Config: config},
			{
				ResourceName:            "portainer_shared_git_credential.g",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"password"},
			},
		},
	})
}

// fakePatternGitCredential is one shared git credential (id 1) that stores
// urlPattern; ignore makes it drop the field, as releases without it do.
type fakePatternGitCredential struct {
	mu      sync.Mutex
	exists  bool
	cred    map[string]interface{}
	pattern string
}

func newFakePatternGitCredential(m *MockServer, ignore bool) *fakePatternGitCredential {
	f := &fakePatternGitCredential{}
	store := func(r *http.Request) {
		var b map[string]interface{}
		_ = json.NewDecoder(r.Body).Decode(&b)
		f.mu.Lock()
		defer f.mu.Unlock()
		f.exists = true
		f.cred = map[string]interface{}{"id": 1, "name": b["name"], "username": b["username"], "authorizationType": b["authorizationType"], "userId": 1}
		if p, ok := b["urlPattern"].(string); ok && !ignore {
			f.pattern = p
		}
	}
	get := func() map[string]interface{} {
		f.mu.Lock()
		defer f.mu.Unlock()
		out := map[string]interface{}{"urlPattern": f.pattern}
		for k, v := range f.cred {
			out[k] = v
		}
		return out
	}
	m.On("POST", "/cloud/gitcredentials", func(w http.ResponseWriter, r *http.Request) {
		store(r)
		RespondJSON(http.StatusOK, map[string]interface{}{"gitCredential": get()})(w, r)
	})
	m.On("PUT", "/cloud/gitcredentials/1", func(w http.ResponseWriter, r *http.Request) { store(r); w.WriteHeader(http.StatusNoContent) })
	m.On("GET", "/cloud/gitcredentials/1", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		exists := f.exists
		f.mu.Unlock()
		if !exists {
			RespondString(http.StatusNotFound, "application/json", `{"message":"not found"}`)(w, r)
			return
		}
		RespondJSON(http.StatusOK, get())(w, r)
	})
	m.On("DELETE", "/cloud/gitcredentials/1", func(w http.ResponseWriter, _ *http.Request) {
		f.mu.Lock()
		f.exists = false
		f.mu.Unlock()
		w.WriteHeader(http.StatusNoContent)
	})
	return f
}

// TestSharedGitCredentialResource_URLPattern: url_pattern is sent on create
// and update only when configured, read back while managed (drift shows).
func TestSharedGitCredentialResource_URLPattern(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	fake := newFakePatternGitCredential(mock, false)
	cfg := func(pattern string) string {
		attr := ""
		if pattern != "" {
			attr = fmt.Sprintf("url_pattern = %q", pattern)
		}
		return mockProviderConfig(mock) + fmt.Sprintf(`
resource "portainer_shared_git_credential" "c" {
  name     = "c"
  username = "u"
  password = "p"
  %s
}`, attr)
	}
	patternSent := func(method, path string, want interface{}) resource.TestCheckFunc {
		return func(*terraform.State) error {
			p := gsbBody(t, mock, method, path)
			if got, ok := p["urlPattern"]; (want == nil && ok) || (want != nil && got != want) {
				return fmt.Errorf("%s %s urlPattern: want %v, got %v (present=%v)", method, path, want, got, ok)
			}
			return nil
		}
	}
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{
				Config: cfg("github.com/portainer/*"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("portainer_shared_git_credential.c", "url_pattern", "github.com/portainer/*"),
					patternSent("POST", "/cloud/gitcredentials", "github.com/portainer/*"),
				),
			},
			{
				PreConfig: func() {
					fake.mu.Lock()
					fake.pattern = "gitlab.com/*"
					fake.mu.Unlock()
				},
				Config:             cfg("github.com/portainer/*"),
				PlanOnly:           true,
				ExpectNonEmptyPlan: true,
			},
			{
				Config: cfg("github.com/*"),
				Check:  patternSent("PUT", "/cloud/gitcredentials/1", "github.com/*"),
			},
			// An import adopts the pattern the credential has.
			{
				ResourceName:            "portainer_shared_git_credential.c",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"password"},
			},
			{
				Config: cfg(""),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckNoResourceAttr("portainer_shared_git_credential.c", "url_pattern"),
					patternSent("PUT", "/cloud/gitcredentials/1", nil),
				),
			},
		},
	})
}

// TestSharedGitCredentialResource_URLPatternIgnored: a server that drops
// url_pattern fails the apply with the requirement.
func TestSharedGitCredentialResource_URLPatternIgnored(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	newFakePatternGitCredential(mock, true)
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{{
			Config: mockProviderConfig(mock) + `
resource "portainer_shared_git_credential" "c" {
  name        = "c"
  username    = "u"
  password    = "p"
  url_pattern = "github.com/*"
}`,
			ExpectError: regexp.MustCompile(`did not store url_pattern`),
		}},
	})
}

// TestSharedGitCredentialResource_URLPattern_UpgradeFromV300: a credential
// with a URL pattern plans no change after upgrading, without the attribute.
func TestSharedGitCredentialResource_URLPattern_UpgradeFromV300(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	fake := newFakePatternGitCredential(mock, false)
	fake.pattern = "github.com/*"
	config := mockProviderConfig(mock) + `
resource "portainer_shared_git_credential" "c" {
  name     = "c"
  username = "u"
  password = "p"
}`
	resource.Test(t, resource.TestCase{
		IsUnitTest: true,
		Steps: []resource.TestStep{
			{ExternalProviders: previousRelease, Config: config},
			{
				ProtoV5ProviderFactories: testProtoV5Factories,
				Config:                   config,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
		},
	})
}
