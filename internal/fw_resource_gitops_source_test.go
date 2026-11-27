package internal

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"sync"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/tfversion"
)

// fakeGitopsSources is a stateful stand-in for Portainer's GitOps source API.
// Like Portainer it stores the URL normalized, derives a name from it and
// polls every minute unless told otherwise. accessBody, when set, replaces
// the answer of the access update (Portainer 2.45 returned the stored Source).
type fakeGitopsSources struct {
	mu         sync.Mutex
	nextID     int
	sources    map[int]*gitopsSourceDetail
	accessBody interface{}
}

func newFakeGitopsSources(m *MockServer) *fakeGitopsSources {
	f := &fakeGitopsSources{nextID: 11, sources: map[int]*gitopsSourceDetail{}}
	m.On("POST", "/gitops/sources/git", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			URL            string `json:"url"`
			Name           string `json:"name"`
			Interval       string `json:"interval"`
			TLSSkipVerify  bool   `json:"tlsSkipVerify"`
			Public         bool   `json:"public"`
			UserAccesses   []int  `json:"userAccesses"`
			TeamAccesses   []int  `json:"teamAccesses"`
			Authentication *struct {
				Username string `json:"username"`
			} `json:"authentication"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		f.mu.Lock()
		defer f.mu.Unlock()
		f.nextID++
		source := &gitopsSourceDetail{
			ID: f.nextID, Name: body.Name, URL: normalizeGitURL(body.URL), Type: "git",
			Status: "healthy", Interval: body.Interval, LastSync: 1756900000,
		}
		if source.Name == "" {
			source.Name = "infra"
		}
		if source.Interval == "" {
			source.Interval = "1m"
		}
		source.Connection.TLSSkipVerify = body.TLSSkipVerify
		if body.Authentication != nil {
			source.Connection.Authentication.Username = body.Authentication.Username
		}
		source.Access.Public = body.Public
		source.Access.Users = body.UserAccesses
		source.Access.Teams = body.TeamAccesses
		f.sources[source.ID] = source
		RespondJSON(http.StatusCreated, map[string]interface{}{"id": source.ID})(w, r)
	})
	for id := 12; id <= 14; id++ {
		id := id
		base := fmt.Sprintf("/gitops/sources/%d", id)
		notFound := RespondString(http.StatusNotFound, "application/json", `{"message":"source not found"}`)
		m.On("GET", base, func(w http.ResponseWriter, r *http.Request) {
			f.mu.Lock()
			defer f.mu.Unlock()
			source, ok := f.sources[id]
			if !ok {
				notFound(w, r)
				return
			}
			RespondJSON(http.StatusOK, source)(w, r)
		})
		m.On("PUT", base, func(w http.ResponseWriter, r *http.Request) {
			var body struct {
				URL            string `json:"url"`
				Name           string `json:"name"`
				Interval       string `json:"interval"`
				TLSSkipVerify  bool   `json:"tlsSkipVerify"`
				Authentication *struct {
					Username string `json:"username"`
				} `json:"authentication"`
			}
			_ = json.NewDecoder(r.Body).Decode(&body)
			f.mu.Lock()
			defer f.mu.Unlock()
			source, ok := f.sources[id]
			if !ok {
				notFound(w, r)
				return
			}
			source.URL, source.Name, source.Interval = normalizeGitURL(body.URL), body.Name, body.Interval
			source.Connection.TLSSkipVerify = body.TLSSkipVerify
			source.Connection.Authentication.Username = ""
			if body.Authentication != nil {
				source.Connection.Authentication.Username = body.Authentication.Username
			}
			RespondJSON(http.StatusOK, source)(w, r)
		})
		m.On("PUT", base+"/access", func(w http.ResponseWriter, r *http.Request) {
			var body struct {
				Public bool  `json:"public"`
				Users  []int `json:"users"`
				Teams  []int `json:"teams"`
			}
			_ = json.NewDecoder(r.Body).Decode(&body)
			f.mu.Lock()
			defer f.mu.Unlock()
			source, ok := f.sources[id]
			if !ok {
				notFound(w, r)
				return
			}
			source.Access.Public, source.Access.Users, source.Access.Teams = body.Public, body.Users, body.Teams
			if f.accessBody != nil {
				RespondJSON(http.StatusOK, f.accessBody)(w, r)
				return
			}
			RespondJSON(http.StatusOK, source)(w, r)
		})
		m.On("DELETE", base, func(w http.ResponseWriter, r *http.Request) {
			f.mu.Lock()
			defer f.mu.Unlock()
			if _, ok := f.sources[id]; !ok {
				notFound(w, r)
				return
			}
			delete(f.sources, id)
			w.WriteHeader(http.StatusNoContent)
		})
	}
	return f
}

func gitopsSourceConfig(m *MockServer, body string) string {
	return mockProviderConfig(m) + `
resource "portainer_gitops_source" "s" {
` + body + `
}`
}

const gitopsSourceFullBody = `
  url                 = "https://github.com/acme/infra.git"
  name                = "infra"
  interval            = "5m"
  username            = "ci"
  password            = "token"
  tls_skip_verify     = true
  administrators_only = true
  user_accesses       = [7, 9]
  team_accesses       = [4]
  public              = %v
`

// TestGitopsSourceResource_Lifecycle pins the create payload's camelCase
// keys, the state the follow-up read fills in, and the two-endpoint update:
// the git connection goes to the source, access control to its own endpoint,
// each only when its own arguments changed.
func TestGitopsSourceResource_Lifecycle(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	newFakeGitopsSources(mock)

	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		CheckDestroy: gitopsCheck(func() error {
			if mock.FindRequest("DELETE", "/gitops/sources/12") == nil {
				return fmt.Errorf("expected DELETE /gitops/sources/12")
			}
			return nil
		}),
		Steps: []resource.TestStep{
			{
				Config: gitopsSourceConfig(mock, fmt.Sprintf(gitopsSourceFullBody, false)),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("portainer_gitops_source.s", "id", "12"),
					resource.TestCheckResourceAttr("portainer_gitops_source.s", "status", "healthy"),
					resource.TestCheckResourceAttr("portainer_gitops_source.s", "type", "git"),
					resource.TestCheckResourceAttr("portainer_gitops_source.s", "last_sync", "1756900000"),
					resource.TestCheckResourceAttr("portainer_gitops_source.s", "username", "ci"),
					// Portainer stores the URL normalized; the configured
					// spelling stays.
					resource.TestCheckResourceAttr("portainer_gitops_source.s", "url", "https://github.com/acme/infra.git"),
					gitopsCheck(func() error {
						payload := gitopsLastPayload(t, mock, "POST", "/gitops/sources/git")
						if payload["url"] != "https://github.com/acme/infra.git" || payload["name"] != "infra" || payload["interval"] != "5m" {
							return fmt.Errorf("payload identity mismatch: %v", payload)
						}
						if payload["tlsSkipVerify"] != true || payload["administratorsOnly"] != true || payload["public"] != false {
							return fmt.Errorf("payload flags mismatch: %v", payload)
						}
						auth, ok := payload["authentication"].(map[string]interface{})
						if !ok || auth["username"] != "ci" || auth["password"] != "token" {
							return fmt.Errorf("payload.authentication mismatch: %v", payload["authentication"])
						}
						if len(payload["userAccesses"].([]interface{})) != 2 || len(payload["teamAccesses"].([]interface{})) != 1 {
							return fmt.Errorf("payload accesses: %v", payload)
						}
						return nil
					}),
				),
			},
			{
				Config:   gitopsSourceConfig(mock, fmt.Sprintf(gitopsSourceFullBody, false)),
				PlanOnly: true,
			},
			{
				Config: gitopsSourceConfig(mock, fmt.Sprintf(gitopsSourceFullBody, true)),
				Check: gitopsCheck(func() error {
					if n := gitopsRequestCount(mock, "PUT", "/gitops/sources/12"); n != 0 {
						return fmt.Errorf("only access changed, yet the source was updated %d times", n)
					}
					payload := gitopsLastPayload(t, mock, "PUT", "/gitops/sources/12/access")
					if payload["public"] != true || len(payload["users"].([]interface{})) != 2 || len(payload["teams"].([]interface{})) != 1 {
						return fmt.Errorf("access payload: %v", payload)
					}
					if _, present := payload["url"]; present {
						return fmt.Errorf("the access payload must not carry connection fields")
					}
					return nil
				}),
			},
			{
				// Spelled differently, the same repository: Terraform plans
				// the new spelling into state, but Portainer is not called.
				Config: gitopsSourceConfig(mock, `url = "https://GitHub.com/acme/infra/"`+"\n"+`name = "infra"`+"\n"+`interval = "5m"`+"\n"+`username = "ci"`+"\n"+`password = "token"`+"\n"+`tls_skip_verify = true`+"\n"+`administrators_only = true`+"\n"+`user_accesses = [7, 9]`+"\n"+`team_accesses = [4]`+"\n"+`public = true`),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("portainer_gitops_source.s", "url", "https://GitHub.com/acme/infra/"),
					gitopsCheck(func() error {
						if n := gitopsRequestCount(mock, "PUT", "/gitops/sources/12"); n != 0 {
							return fmt.Errorf("a respelled URL must not update the source (%d calls)", n)
						}
						return nil
					}),
				),
			},
			{
				Config: gitopsSourceConfig(mock, `
  url                 = "https://github.com/acme/apps.git"
  administrators_only = true`),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction("portainer_gitops_source.s", plancheck.ResourceActionUpdate)},
				},
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckNoResourceAttr("portainer_gitops_source.s", "username"),
					resource.TestCheckResourceAttr("portainer_gitops_source.s", "name", "infra"),
					resource.TestCheckResourceAttr("portainer_gitops_source.s", "public", "true"),
					gitopsCheck(func() error {
						payload := gitopsLastPayload(t, mock, "PUT", "/gitops/sources/12")
						if payload["url"] != "https://github.com/acme/apps.git" || payload["name"] != "infra" || payload["interval"] != "5m" {
							return fmt.Errorf("source update payload: %v", payload)
						}
						if _, present := payload["public"]; present {
							return fmt.Errorf("access control must not be sent on the source update payload")
						}
						if _, present := payload["authentication"]; present {
							return fmt.Errorf("no credentials configured, none must be sent: %v", payload["authentication"])
						}
						if n := gitopsRequestCount(mock, "PUT", "/gitops/sources/12/access"); n != 1 {
							return fmt.Errorf("access did not change, yet it was updated again (%d calls)", n)
						}
						return nil
					}),
				),
			},
			{
				Config: gitopsSourceConfig(mock, `
  url                 = "https://github.com/acme/apps.git"
  administrators_only = true`),
				ResourceName:            "portainer_gitops_source.s",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"administrators_only", "url"},
			},
		},
	})
}

// TestGitopsSourceResource_AdministratorsOnlyForcesNew: Portainer's update API
// has no such field, so a change replaces the source.
func TestGitopsSourceResource_AdministratorsOnlyForcesNew(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	newFakeGitopsSources(mock)
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{Config: gitopsSourceConfig(mock, `url = "https://github.com/acme/infra"`)},
			{
				Config: gitopsSourceConfig(mock, `
  url                 = "https://github.com/acme/infra"
  administrators_only = true`),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction("portainer_gitops_source.s", plancheck.ResourceActionDestroyBeforeCreate)},
				},
				Check: resource.TestCheckResourceAttr("portainer_gitops_source.s", "id", "13"),
			},
		},
	})
}

// TestGitopsSourceResource_NoIDReturned: a response without an ID fails
// loudly instead of leaving a resource pointing at source 0.
func TestGitopsSourceResource_NoIDReturned(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("POST", "/gitops/sources/git", RespondJSON(http.StatusCreated, map[string]interface{}{}))
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{{
			Config:      gitopsSourceConfig(mock, `url = "https://github.com/acme/infra.git"`),
			ExpectError: regexp.MustCompile(`Portainer returned no source ID`),
		}},
	})
}

// TestGitopsSourceResource_GoneOutsideTerraform: a source deleted outside
// Terraform drops out of state instead of failing every plan, and a delete
// answering 404 counts as done.
func TestGitopsSourceResource_GoneOutsideTerraform(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	fake := newFakeGitopsSources(mock)
	config := gitopsSourceConfig(mock, `url = "https://github.com/acme/infra.git"`)
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{Config: config},
			{
				PreConfig: func() {
					fake.mu.Lock()
					delete(fake.sources, 12)
					fake.mu.Unlock()
				},
				Config:             config,
				PlanOnly:           true,
				ExpectNonEmptyPlan: true,
			},
		},
	})
}

// TestGitopsSourceResource_AccessResponseShapes: Portainer 3.0 answers the
// access update with a SourceDetail where 2.45 returned the stored Source. The
// provider reads the source back on its own, so either body must do.
func TestGitopsSourceResource_AccessResponseShapes(t *testing.T) {
	t.Parallel()
	for name, body := range map[string]interface{}{
		"2.45 portainer.Source":    map[string]interface{}{"ID": 12, "Name": "infra", "URL": "https://github.com/acme/infra.git"},
		"3.0 sources.SourceDetail": nil,
	} {
		t.Run(name, func(t *testing.T) {
			mock := NewMockServer(t)
			fake := newFakeGitopsSources(mock)
			fake.accessBody = body
			resource.UnitTest(t, resource.TestCase{
				ProtoV5ProviderFactories: testProtoV5Factories,
				Steps: []resource.TestStep{
					{Config: gitopsSourceConfig(mock, `url = "https://github.com/acme/infra"`)},
					{
						Config: gitopsSourceConfig(mock, `
  url    = "https://github.com/acme/infra"
  public = true`),
						Check: resource.TestCheckResourceAttr("portainer_gitops_source.s", "public", "true"),
					},
				},
			})
		})
	}
}

// TestGitopsSourceResource_WriteOnlyPassword: password_wo is sent and never
// stored; the password argument stays unset in state.
func TestGitopsSourceResource_WriteOnlyPassword(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	newFakeGitopsSources(mock)
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		TerraformVersionChecks:   []tfversion.TerraformVersionCheck{tfversion.SkipBelow(tfversion.Version1_11_0)},
		Steps: []resource.TestStep{
			{
				Config: gitopsSourceConfig(mock, `
  url                 = "https://github.com/acme/infra.git"
  username            = "bot"
  password_wo         = "git-token-3"
  password_wo_version = 1`),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckNoResourceAttr("portainer_gitops_source.s", "password"),
					resource.TestCheckNoResourceAttr("portainer_gitops_source.s", "password_wo"),
					gitopsCheck(func() error {
						auth, _ := gitopsLastPayload(t, mock, "POST", "/gitops/sources/git")["authentication"].(map[string]interface{})
						if auth["password"] != "git-token-3" {
							return fmt.Errorf("authentication.password: expected the write-only value, got %v", auth["password"])
						}
						return nil
					}),
				),
			},
			{
				// A new version sends the new value.
				Config: gitopsSourceConfig(mock, `
  url                 = "https://github.com/acme/infra.git"
  username            = "bot"
  password_wo         = "git-token-4"
  password_wo_version = 2`),
				Check: gitopsCheck(func() error {
					auth, _ := gitopsLastPayload(t, mock, "PUT", "/gitops/sources/12")["authentication"].(map[string]interface{})
					if auth["password"] != "git-token-4" {
						return fmt.Errorf("authentication.password: expected the new write-only value, got %v", auth["password"])
					}
					return nil
				}),
			},
		},
	})
}

// TestGitopsSourceResource_PasswordConflicts: password and password_wo are
// alternatives, and password_wo needs its version.
func TestGitopsSourceResource_PasswordConflicts(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	for config, want := range map[string]string{
		`
  url                 = "https://github.com/acme/infra.git"
  password            = "a"
  password_wo         = "b"
  password_wo_version = 2`: `cannot be specified when`,
		`
  url         = "https://github.com/acme/infra.git"
  password_wo = "b"`: `password_wo_version`,
	} {
		resource.UnitTest(t, resource.TestCase{
			ProtoV5ProviderFactories: testProtoV5Factories,
			TerraformVersionChecks:   []tfversion.TerraformVersionCheck{tfversion.SkipBelow(tfversion.Version1_11_0)},
			Steps: []resource.TestStep{{
				Config:      gitopsSourceConfig(mock, config),
				PlanOnly:    true,
				ExpectError: regexp.MustCompile(want),
			}},
		})
	}
}

// TestGitopsSource_UpgradeFromV300: state written by v3.0.0 (SDKv2) against
// the same fake plans no change with this build, for configurations that set
// and that omit the optional arguments.
func TestGitopsSource_UpgradeFromV300(t *testing.T) {
	t.Parallel()
	for name, body := range map[string]string{
		"minimal": `url = "https://github.com/acme/infra"`,
		"everything": `
  url                 = "https://github.com/acme/infra"
  name                = "infra"
  interval            = "5m"
  username            = "ci"
  password            = "token"
  tls_skip_verify     = true
  administrators_only = true
  user_accesses       = [7, 9]
  team_accesses       = [4]
  public              = true`,
		"defaults spelled out": `
  url                 = "https://github.com/acme/infra"
  tls_skip_verify     = false
  administrators_only = false
  public              = false
  user_accesses       = []
  team_accesses       = []`,
	} {
		t.Run(name, func(t *testing.T) {
			mock := NewMockServer(t)
			newFakeGitopsSources(mock)
			config := gitopsSourceConfig(mock, body)
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
		})
	}
}

// TestGitopsSource_UpgradeFromV300RespelledURL documents the one difference
// the upgrade cannot hide: v3.0.0 stored the URL as Portainer normalized it and
// suppressed the difference to a configuration spelling it otherwise (with
// .git, say). The framework cannot plan a value other than the configured one,
// so the first plan records the configured spelling - an update that calls no
// Portainer API - and is stable from then on.
func TestGitopsSource_UpgradeFromV300RespelledURL(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	newFakeGitopsSources(mock)
	config := gitopsSourceConfig(mock, `url = "https://github.com/acme/infra.git"`)
	resource.Test(t, resource.TestCase{
		IsUnitTest: true,
		Steps: []resource.TestStep{
			{ExternalProviders: previousRelease, Config: config},
			{
				ProtoV5ProviderFactories: testProtoV5Factories,
				Config:                   config,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction("portainer_gitops_source.s", plancheck.ResourceActionUpdate)},
				},
				Check: gitopsCheck(func() error {
					if n := gitopsRequestCount(mock, "PUT", "/gitops/sources/12"); n != 0 {
						return fmt.Errorf("a respelled URL must not update the source (%d calls)", n)
					}
					return nil
				}),
			},
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

// TestAccGitopsSourceResource needs a Business Edition instance and a
// reachable public repository.
func TestAccGitopsSourceResource(t *testing.T) {
	if os.Getenv("PORTAINER_ACC_BE") != "1" {
		t.Skip("GitOps sources need Portainer Business Edition: set PORTAINER_ACC_BE=1")
	}
	config := accProvider() + `
resource "portainer_gitops_source" "s" {
  url      = "https://github.com/portainer/templates.git"
  interval = "10m"
}`
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { accPreCheck(t) },
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{Config: config, Check: resource.TestCheckResourceAttr("portainer_gitops_source.s", "type", "git")},
			{Config: config, PlanOnly: true},
			{ResourceName: "portainer_gitops_source.s", ImportState: true, ImportStateVerify: true, ImportStateVerifyIgnore: []string{"administrators_only", "url"}},
		},
	})
}
