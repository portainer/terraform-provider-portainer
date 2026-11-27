package internal

import (
	"fmt"
	"net/http"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

func TestParseUserGitCredentialID(t *testing.T) {
	t.Parallel()
	tests := []struct {
		input        string
		wantUserID   int
		wantCredID   int
		errSubstring string
	}{
		{input: "1:2", wantUserID: 1, wantCredID: 2},
		{input: "12345:67890", wantUserID: 12345, wantCredID: 67890},
		{input: "0:0"},
		{input: "-1:2", wantUserID: -1, wantCredID: 2},
		{input: "12345", errSubstring: "unexpected format"},
		{input: "", errSubstring: "unexpected format"},
		{input: "abc:123", errSubstring: "invalid user ID"},
		{input: "123:abc", errSubstring: "invalid credential ID"},
		{input: "abc:def", errSubstring: "invalid user ID"},
		{input: "1:2:3", errSubstring: "invalid credential ID"},
		{input: ":", errSubstring: "invalid user ID"},
	}
	for _, tt := range tests {
		userID, credID, err := parseUserGitCredentialID(tt.input)
		if tt.errSubstring != "" {
			if err == nil || !strings.Contains(err.Error(), tt.errSubstring) {
				t.Errorf("%q: expected an error containing %q, got %v", tt.input, tt.errSubstring, err)
			}
			continue
		}
		if err != nil || userID != tt.wantUserID || credID != tt.wantCredID {
			t.Errorf("%q: got %d, %d, %v", tt.input, userID, credID, err)
		}
	}
}

func userGitCredentialConfig(mock *MockServer, username, password string) string {
	return mockProviderConfig(mock) + fmt.Sprintf(`
resource "portainer_user_git_credential" "g" {
  user_id  = 5
  name     = "mine"
  username = %q
  password = %q
}
`, username, password)
}

// TestUserGitCredentialResource_Lifecycle: create under the user, the
// composite ID, update with PUT, import by "<user_id>:<credential_id>",
// destroy.
// TestUserGitCredentialResource_Removed: on Business Edition 2.43 and later
// the API 404s; the error says it was removed and names the replacement.
func TestUserGitCredentialResource_Removed(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		edition, version string
		want             *regexp.Regexp
	}{
		{"EE", "3.1.0", regexp.MustCompile(`not\s+available\s+on\s+Portainer\s+2\.43\s+or\s+later[\s\S]*portainer_shared_git_credential`)},
		{"CE", "2.39.9", regexp.MustCompile(`requires\s+Portainer\s+Business\s+Edition\s+2\.42\s+or\s+earlier;\s+this\s+server\s+is\s+Portainer\s+CE\s+2\.39\.9`)},
		{"EE", "2.42.0", regexp.MustCompile(`failed to create user git credential: POST[\s\S]*failed with\s+status 404`)},
	} {
		t.Run(tc.edition+"-"+tc.version, func(t *testing.T) {
			t.Parallel()
			mock := NewMockServer(t)
			mock.On("POST", "/users/5/gitcredentials", RespondString(http.StatusNotFound, "text/plain", "404 page not found"))
			mock.On("GET", "/system/version", RespondJSON(http.StatusOK, map[string]interface{}{"ServerVersion": tc.version, "ServerEdition": tc.edition}))
			resource.UnitTest(t, resource.TestCase{
				ProtoV5ProviderFactories: testProtoV5Factories,
				Steps:                    []resource.TestStep{{Config: userGitCredentialConfig(mock, "robot", "token"), ExpectError: tc.want}},
			})
		})
	}
}

func TestUserGitCredentialResource_Lifecycle(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	fake := newFakeGitCredentials(mock, "/users/5/gitcredentials", 5, 2)

	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		CheckDestroy: func(*terraform.State) error {
			if mock.FindRequest("DELETE", "/users/5/gitcredentials/1") == nil {
				return fmt.Errorf("expected DELETE /users/5/gitcredentials/1")
			}
			return nil
		},
		Steps: []resource.TestStep{
			{
				Config: userGitCredentialConfig(mock, "robot", "token"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("portainer_user_git_credential.g", "id", "5:1"),
					resource.TestCheckResourceAttr("portainer_user_git_credential.g", "credential_id", "1"),
					resource.TestCheckResourceAttr("portainer_user_git_credential.g", "authorization_type", "0"),
					func(*terraform.State) error {
						var payload map[string]interface{}
						post := mock.FindRequest("POST", "/users/5/gitcredentials")
						if post == nil || post.DecodeJSON(&payload) != nil {
							return fmt.Errorf("expected a JSON POST")
						}
						if payload["name"] != "mine" || payload["password"] != "token" || payload["authorizationType"] != float64(0) {
							return fmt.Errorf("unexpected payload %v", payload)
						}
						return nil
					},
				),
			},
			{
				Config: userGitCredentialConfig(mock, "robot2", "token2"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction("portainer_user_git_credential.g", plancheck.ResourceActionUpdate)},
				},
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("portainer_user_git_credential.g", "username", "robot2"),
					resource.TestCheckResourceAttr("portainer_user_git_credential.g", "credential_id", "1"),
					func(*terraform.State) error {
						if fake.password(1) != "token2" {
							return fmt.Errorf("the new password was not sent with PUT")
						}
						return nil
					},
				),
			},
			{
				ResourceName:            "portainer_user_git_credential.g",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"password"},
			},
		},
	})
}

func TestUserGitCredentialResource_ImportBadID(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	for id, want := range map[string]string{
		"12345": "expected <user_id>:<credential_id>",
		"abc:1": "invalid user ID",
		"5:abc": "invalid credential ID",
	} {
		resource.UnitTest(t, resource.TestCase{
			ProtoV5ProviderFactories: testProtoV5Factories,
			Steps: []resource.TestStep{{
				Config:        userGitCredentialConfig(mock, "u", "p"),
				ResourceName:  "portainer_user_git_credential.g",
				ImportState:   true,
				ImportStateId: id,
				ExpectError:   regexp.MustCompile(regexp.QuoteMeta(want)),
			}},
		})
	}
}

func TestUserGitCredentialResource_CreateHTTPError(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("POST", "/users/5/gitcredentials", RespondString(http.StatusForbidden, "application/json", `{"message":"forbidden"}`))
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{{
			Config:      userGitCredentialConfig(mock, "u", "p"),
			ExpectError: regexp.MustCompile(`failed to create user git credential`),
		}},
	})
}

func TestUserGitCredentialResource_GoneOutsideTerraform(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	fake := newFakeGitCredentials(mock, "/users/5/gitcredentials", 5, 2)
	config := userGitCredentialConfig(mock, "u", "p")
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

func TestUserGitCredentialResource_UpgradeFromV300(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	newFakeGitCredentials(mock, "/users/5/gitcredentials", 5, 2)
	config := userGitCredentialConfig(mock, "u", "p") + `
resource "portainer_user_git_credential" "token" {
  user_id            = 5
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

// TestAccUserGitCredentialResource needs Business Edition 2.42 or earlier
// (Business Edition 2.43 removed the per-user git credential API); it uses
// the admin user (ID 1).
func TestAccUserGitCredentialResource(t *testing.T) {
	if os.Getenv("PORTAINER_ACC_BE") != "1" {
		t.Skip("user git credentials need Portainer Business Edition; set PORTAINER_ACC_BE=1")
	}
	accSkipUnlessAPI(t, "/users/1/gitcredentials", "Portainer Business Edition 2.43 and later have no per-user git credentials")
	config := accProvider() + fmt.Sprintf(`
resource "portainer_user_git_credential" "g" {
  user_id  = 1
  name     = %q
  username = "robot"
  password = "token"
}
`, acctest.RandomWithPrefix("acc-git"))
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { accPreCheck(t) },
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{Config: config},
			{
				ResourceName:            "portainer_user_git_credential.g",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"password"},
			},
		},
	})
}
