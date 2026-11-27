package internal

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// TestUserResource_Lifecycle: create with a team and a generated API key, a
// password change, a team move, imports by ID and by username, destroy.
func TestUserResource_Lifecycle(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	fake := newFakeUsersAPI(mock)
	teamA := fake.addTeam("a")
	teamB := fake.addTeam("b")

	cfg := func(password string, team int64) string {
		return mockProviderConfig(mock) + fmt.Sprintf(`
resource "portainer_user" "u" {
  username         = "alice"
  password         = %q
  team_id          = %d
  generate_api_key = true
}`, password, team)
	}
	importIgnore := []string{"password", "generate_api_key", "api_key_description", "api_key_raw", "ldap_user", "team_id"}

	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{
				Config: cfg("pw-1234567890", teamA),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("portainer_user.u", "id", "3"),
					resource.TestCheckResourceAttr("portainer_user.u", "role", "2"),
					resource.TestCheckResourceAttr("portainer_user.u", "ldap_user", "false"),
					resource.TestCheckResourceAttr("portainer_user.u", "api_key_description", "terraform-generated-api-key"),
					resource.TestCheckResourceAttrSet("portainer_user.u", "api_key_raw"),
					resource.TestCheckResourceAttr("portainer_user.u", "created_authentication_method", "internal"),
					resource.TestCheckResourceAttr("portainer_user.u", "last_authentication_method", ""),
					func(*terraform.State) error {
						post := mock.FindRequest("POST", "/users")
						var body map[string]interface{}
						if err := post.DecodeJSON(&body); err != nil {
							return err
						}
						if body["username"] != "alice" || body["password"] != "pw-1234567890" || body["role"] != float64(2) {
							return fmt.Errorf("unexpected create payload %v", body)
						}
						if fake.membershipCount(3, teamA) != 1 {
							return fmt.Errorf("expected a membership in team %d", teamA)
						}
						// The key is created with the new user's JWT.
						tok := mock.FindRequest("POST", "/users/3/tokens")
						if tok == nil || tok.Headers.Get("Authorization") != "Bearer jwt-3" {
							return fmt.Errorf("expected the token request authenticated as the new user, got %+v", tok)
						}
						return nil
					},
				),
			},
			{
				Config: cfg("pw-new-1234567890", teamB),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction("portainer_user.u", plancheck.ResourceActionUpdate)},
				},
				Check: func(*terraform.State) error {
					pw := mock.FindRequest("PUT", "/users/3/passwd")
					var body map[string]string
					if pw == nil || pw.DecodeJSON(&body) != nil || body["password"] != "pw-1234567890" || body["newPassword"] != "pw-new-1234567890" {
						return fmt.Errorf("unexpected password change %v", body)
					}
					put := mock.FindRequest("PUT", "/users/3")
					var upd map[string]interface{}
					if put == nil || put.DecodeJSON(&upd) != nil || upd["newPassword"] != nil || upd["password"] != nil || upd["useCache"] != true {
						return fmt.Errorf("unexpected user update %v", upd)
					}
					if fake.membershipCount(3, teamA) != 0 || fake.membershipCount(3, teamB) != 1 {
						return fmt.Errorf("expected the membership moved from team %d to %d", teamA, teamB)
					}
					return nil
				},
			},
			{ResourceName: "portainer_user.u", ImportState: true, ImportStateVerify: true, ImportStateVerifyIgnore: importIgnore},
			{ResourceName: "portainer_user.u", ImportState: true, ImportStateId: "alice", ImportStateVerify: true, ImportStateVerifyIgnore: importIgnore},
		},
	})
	if len(requestsTo(mock, "DELETE", "/users/3")) != 1 {
		t.Error("expected the user deleted on destroy")
	}
}

// TestUserResource_LDAP: an LDAP user is created with a null password.
func TestUserResource_LDAP(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	newFakeUsersAPI(mock)
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{{
			Config: mockProviderConfig(mock) + `resource "portainer_user" "u" {
  username  = "ldapuser"
  ldap_user = true
}`,
			Check: resource.ComposeTestCheckFunc(
				resource.TestCheckNoResourceAttr("portainer_user.u", "password"),
				resource.TestCheckResourceAttr("portainer_user.u", "created_authentication_method", "ldap"),
				func(*terraform.State) error {
					post := mock.FindRequest("POST", "/users")
					var body map[string]interface{}
					if err := post.DecodeJSON(&body); err != nil {
						return err
					}
					if v, ok := body["password"]; !ok || v != nil {
						return fmt.Errorf("expected password sent as null, got %v", body)
					}
					return nil
				},
			),
		}},
	})
}

func TestUserResource_PasswordGuards(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	newFakeUsersAPI(mock)
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{
				Config: mockProviderConfig(mock) + `resource "portainer_user" "u" {
  username = "x"
  ldap_user = true
  password = "pw"
}`,
				ExpectError: regexp.MustCompile(`cannot set password for LDAP user`),
			},
			{
				Config:      mockProviderConfig(mock) + `resource "portainer_user" "u" { username = "x" }`,
				ExpectError: regexp.MustCompile(`password is required for non-LDAP user`),
			},
			{
				Config: mockProviderConfig(mock) + `resource "portainer_user" "u" {
  username = "x"
  ldap_user = true
  generate_api_key = true
}`,
				ExpectError: regexp.MustCompile(`password must be set to generate API key`),
			},
		},
	})
	if mock.FindRequest("POST", "/auth") != nil {
		t.Error("no key may be requested without a password")
	}
}

// TestUserResource_TeamIDRequiresStandardRole: the user is created, then the
// team_id guard fires.
func TestUserResource_TeamIDRequiresStandardRole(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	fake := newFakeUsersAPI(mock)
	team := fake.addTeam("t")
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{{
			Config: mockProviderConfig(mock) + fmt.Sprintf(`resource "portainer_user" "u" {
  username = "x"
  password = "pw"
  role = 1
  team_id = %d
}`, team),
			ExpectError: regexp.MustCompile(`team_id can only be used with standard users`),
		}},
	})
	if mock.FindRequest("POST", "/team_memberships") != nil {
		t.Error("no membership may be created for an administrator")
	}
}

// TestUserResource_AdoptsExistingUser: a user that already exists is updated
// instead of created.
func TestUserResource_AdoptsExistingUser(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	fake := newFakeUsersAPI(mock)
	fake.addUser("bob", "old", 2)
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{{
			Config: mockProviderConfig(mock) + `resource "portainer_user" "u" {
  username = "bob"
  password = "pw-1234567890"
  role     = 1
}`,
			Check: resource.ComposeTestCheckFunc(
				resource.TestCheckResourceAttr("portainer_user.u", "id", "1"),
				resource.TestCheckResourceAttr("portainer_user.u", "role", "1"),
				func(*terraform.State) error {
					if mock.FindRequest("POST", "/users") != nil {
						return fmt.Errorf("an existing user must not be created again")
					}
					if mock.FindRequest("PUT", "/users/1") == nil {
						return fmt.Errorf("expected the existing user updated")
					}
					// The old password is unknown, so it is not changed.
					if mock.FindRequest("PUT", "/users/1/passwd") != nil {
						return fmt.Errorf("no password change without the current password")
					}
					return nil
				},
			),
		}},
	})
}

// TestUserResource_TeamIDTracksOnlyManagedTeam: another membership of the user
// does not show up as team_id, and a removed managed membership plans its
// re-creation.
func TestUserResource_TeamIDTracksOnlyManagedTeam(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	fake := newFakeUsersAPI(mock)
	team := fake.addTeam("managed")
	other := fake.addTeam("other")
	withTeam := mockProviderConfig(mock) + fmt.Sprintf(`resource "portainer_user" "u" {
  username = "carol"
  password = "pw-1234567890"
  team_id  = %d
}`, team)
	withoutTeam := mockProviderConfig(mock) + `resource "portainer_user" "u" {
  username = "carol"
  password = "pw-1234567890"
}`

	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{Config: withoutTeam},
			{
				PreConfig: func() { fake.addMembership(3, other, 2) },
				Config:    withoutTeam,
				PlanOnly:  true,
			},
			{Config: withTeam},
			{
				PreConfig: func() {
					fake.mu.Lock()
					for id, m := range fake.memberships {
						if m.TeamID == team {
							delete(fake.memberships, id)
						}
					}
					fake.mu.Unlock()
				},
				Config:             withTeam,
				PlanOnly:           true,
				ExpectNonEmptyPlan: true,
			},
		},
	})
}

// TestUserResource_AuthenticationMethods: the Portainer 3.0 fields map to
// names, and a server that omits them leaves the attributes empty.
func TestUserResource_AuthenticationMethods(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	fake := newFakeUsersAPI(mock)
	config := mockProviderConfig(mock) + `resource "portainer_user" "u" {
  username = "dave"
  password = "pw-1234567890"
}`
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{
				Config: config,
				Check:  resource.TestCheckResourceAttr("portainer_user.u", "created_authentication_method", "internal"),
			},
			{
				PreConfig: func() {
					fake.mu.Lock()
					fake.users[1].CreatedAuthenticationMethod = 2
					fake.users[1].LastAuthenticationMethod = 3
					fake.mu.Unlock()
				},
				RefreshState: true,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("portainer_user.u", "created_authentication_method", "ldap"),
					resource.TestCheckResourceAttr("portainer_user.u", "last_authentication_method", "oauth"),
				),
			},
			{
				// Before 3.0 the fields are absent.
				PreConfig: func() {
					fake.mu.Lock()
					fake.users[1].CreatedAuthenticationMethod = 0
					fake.users[1].LastAuthenticationMethod = 0
					fake.mu.Unlock()
				},
				RefreshState: true,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("portainer_user.u", "created_authentication_method", ""),
					resource.TestCheckResourceAttr("portainer_user.u", "last_authentication_method", ""),
				),
			},
		},
	})
}

// TestUserResource_GoneOutsideTerraform: a deleted user is planned again.
func TestUserResource_GoneOutsideTerraform(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	fake := newFakeUsersAPI(mock)
	config := mockProviderConfig(mock) + `resource "portainer_user" "u" {
  username = "erin"
  password = "pw-1234567890"
}`
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{Config: config},
			{
				PreConfig: func() {
					fake.mu.Lock()
					delete(fake.users, 1)
					fake.mu.Unlock()
				},
				Config:             config,
				PlanOnly:           true,
				ExpectNonEmptyPlan: true,
			},
		},
	})
}

func TestUserResource_PasswordChangeError(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	fake := newFakeUsersAPI(mock)
	cfg := func(pw string) string {
		return mockProviderConfig(mock) + fmt.Sprintf(`resource "portainer_user" "u" {
  username = "frank"
  password = %q
  role     = 2
}`, pw)
	}
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{Config: cfg("pw-1234567890")},
			{
				PreConfig: func() {
					fake.mu.Lock()
					fake.users[1].Password = "changed elsewhere"
					fake.mu.Unlock()
				},
				Config:      cfg("pw-new-1234567890"),
				ExpectError: regexp.MustCompile(`failed to update password`),
			},
		},
	})
	if mock.FindRequest("PUT", "/users/1") != nil {
		t.Error("the user must not be updated after a failed password change")
	}
}

func TestUserResource_ImportUnknownUsername(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	newFakeUsersAPI(mock)
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{{
			Config: mockProviderConfig(mock) + `resource "portainer_user" "u" {
  username = "ghost"
  password = "pw"
}`,
			ResourceName:  "portainer_user.u",
			ImportState:   true,
			ImportStateId: "ghost",
			ExpectError:   regexp.MustCompile(`user ghost not found`),
		}},
	})
}

func TestUserResource_ListError(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("GET", "/users", RespondJSON(500, map[string]string{"message": "boom"}))
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{{
			Config: mockProviderConfig(mock) + `resource "portainer_user" "u" {
  username = "x"
  password = "pw"
}`,
			ExpectError: regexp.MustCompile(`failed to list users`),
		}},
	})
	if mock.FindRequest("POST", "/users") != nil {
		t.Error("no user may be created after a failed list")
	}
}

// TestUserResource_UpgradeFromV300: users written by v3.0.0 (SDKv2) plan no
// change with this build, with and without the optional attributes.
func TestUserResource_UpgradeFromV300(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	fake := newFakeUsersAPI(mock)
	team := fake.addTeam("t")
	config := mockProviderConfig(mock) + fmt.Sprintf(`
resource "portainer_user" "minimal" {
  username = "minimal"
  password = "pw-1234567890"
}

resource "portainer_user" "ldap" {
  username  = "ldap"
  ldap_user = true
}

resource "portainer_user" "full" {
  username            = "full"
  password            = "pw-1234567890"
  role                = 2
  team_id             = %d
  generate_api_key    = true
  api_key_description = "ci"
}`, team)
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

// TestAccUserResource_Basic runs against a real Portainer.
func TestAccUserResource_Basic(t *testing.T) {
	name := acctest.RandomWithPrefix("acc-user")
	config := fmt.Sprintf(`resource "portainer_user" "u" {
  username = %q
  password = "Acc-Test-Password-123"
}`, name)
	resource.Test(t, resource.TestCase{
		PreCheck: func() { accPreCheck(t) },
		Steps: []resource.TestStep{
			{ExternalProviders: previousRelease, Config: accProvider() + config},
			{
				ProtoV5ProviderFactories: testProtoV5Factories,
				Config:                   accProvider() + config,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
			{
				ProtoV5ProviderFactories: testProtoV5Factories,
				Config:                   accProvider() + config,
				ResourceName:             "portainer_user.u",
				ImportState:              true,
				ImportStateVerify:        true,
				ImportStateVerifyIgnore:  []string{"password", "generate_api_key", "api_key_description", "api_key_raw", "ldap_user"},
			},
		},
	})
}
