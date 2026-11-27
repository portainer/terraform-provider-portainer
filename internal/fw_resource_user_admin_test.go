package internal

import (
	"fmt"
	"net/http"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// TestUserAdminResource_Bootstrap: the init call carries the credentials and
// the write-only setup token, the returned ID is kept, the token is not, the
// plan settles, and a changed password is only stored.
func TestUserAdminResource_Bootstrap(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	newFakeUsersAPI(mock)
	cfg := func(pw string) string {
		return mockProviderConfig(mock) + fmt.Sprintf(`resource "portainer_user_admin" "a" {
  password    = %q
  setup_token = "tok-abc"
}`, pw)
	}

	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{
				Config: cfg("S3cret!password"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("portainer_user_admin.a", "id", "1"),
					resource.TestCheckResourceAttr("portainer_user_admin.a", "username", "admin"),
					resource.TestCheckResourceAttr("portainer_user_admin.a", "initialized", "true"),
					resource.TestCheckNoResourceAttr("portainer_user_admin.a", "setup_token"),
					func(*terraform.State) error {
						req := mock.FindRequest("POST", "/users/admin/init")
						if req == nil {
							return fmt.Errorf("expected POST /users/admin/init")
						}
						if got := req.Headers.Get("X-Setup-Token"); got != "tok-abc" {
							return fmt.Errorf("X-Setup-Token: expected tok-abc, got %q", got)
						}
						var body map[string]interface{}
						if err := req.DecodeJSON(&body); err != nil || body["username"] != "admin" || body["password"] != "S3cret!password" {
							return fmt.Errorf("unexpected init payload %v (%w)", body, err)
						}
						return nil
					},
				),
			},
			{
				Config: cfg("S3cret!password"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
			{
				Config: cfg("Another!password"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction("portainer_user_admin.a", plancheck.ResourceActionUpdate)},
				},
				Check: func(*terraform.State) error {
					if n := len(requestsTo(mock, "POST", "/users/admin/init")); n != 1 {
						return fmt.Errorf("expected a single init call, got %d", n)
					}
					return nil
				},
			},
			{ResourceName: "portainer_user_admin.a", ImportState: true, ImportStateVerify: true, ImportStateVerifyIgnore: []string{"password", "username", "setup_token"}},
		},
	})
}

// TestUserAdminResource_SkipsInitWhenAdminExists covers an initialized
// Portainer 2.45, which answers the init call with 403 (setup token) rather
// than 409 until it restarts: the existing admin is detected up front.
func TestUserAdminResource_SkipsInitWhenAdminExists(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	fake := newFakeUsersAPI(mock)
	fake.adminInitialized = true
	fake.initStatus = http.StatusForbidden
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{{
			Config: mockProviderConfig(mock) + `resource "portainer_user_admin" "a" { password = "pw-123456789" }`,
			Check: resource.ComposeTestCheckFunc(
				resource.TestCheckResourceAttr("portainer_user_admin.a", "id", "portainer-admin"),
				resource.TestCheckResourceAttr("portainer_user_admin.a", "initialized", "true"),
			),
		}},
	})
	if mock.FindRequest("POST", "/users/admin/init") != nil {
		t.Error("expected no init call on an instance that already has an admin")
	}
}

// TestUserAdminResource_ConflictIsIdempotent: a 409 from the init call (an
// admin created between the check and the call) counts as success.
func TestUserAdminResource_ConflictIsIdempotent(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("GET", "/users/admin/check", func(w http.ResponseWriter, _ *http.Request) {
		// Missing during the create, present afterwards.
		if mock.FindRequest("POST", "/users/admin/init") == nil {
			fakeNotFound(w)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
	mock.On("POST", "/users/admin/init", RespondString(http.StatusConflict, "application/json", `{"message":"admin already initialized"}`))
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{{
			Config: mockProviderConfig(mock) + `resource "portainer_user_admin" "a" { password = "anything" }`,
			Check: resource.ComposeTestCheckFunc(
				resource.TestCheckResourceAttr("portainer_user_admin.a", "id", "portainer-admin"),
				resource.TestCheckResourceAttr("portainer_user_admin.a", "initialized", "true"),
			),
		}},
	})
}

// TestUserAdminResource_ZeroIDFallsBack: a response without an Id gives the
// stable sentinel ID.
func TestUserAdminResource_ZeroIDFallsBack(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("GET", "/users/admin/check", func(w http.ResponseWriter, _ *http.Request) {
		if mock.FindRequest("POST", "/users/admin/init") == nil {
			fakeNotFound(w)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
	mock.On("POST", "/users/admin/init", RespondJSON(http.StatusOK, map[string]interface{}{"Username": "admin"}))
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{{
			Config: mockProviderConfig(mock) + `resource "portainer_user_admin" "a" { password = "pw" }`,
			Check:  resource.TestCheckResourceAttr("portainer_user_admin.a", "id", "portainer-admin"),
		}},
	})
}

func TestUserAdminResource_Errors(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	fake := newFakeUsersAPI(mock)
	fake.initStatus = http.StatusInternalServerError
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{{
			Config:      mockProviderConfig(mock) + `resource "portainer_user_admin" "a" { password = "pw" }`,
			ExpectError: regexp.MustCompile(`failed to initialize admin user`),
		}},
	})

	failing := NewMockServer(t)
	failing.On("GET", "/users/admin/check", RespondJSON(http.StatusInternalServerError, map[string]string{"message": "boom"}))
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{{
			Config:      mockProviderConfig(failing) + `resource "portainer_user_admin" "a" { password = "pw" }`,
			ExpectError: regexp.MustCompile(`failed to check for an admin user`),
		}},
	})
}

// TestUserAdminResource_DropsWhenAdminMissing covers a reset instance:
// without an administrator the bootstrap is planned again.
func TestUserAdminResource_DropsWhenAdminMissing(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	fake := newFakeUsersAPI(mock)
	config := mockProviderConfig(mock) + `resource "portainer_user_admin" "a" { password = "pw-123456789" }`
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{Config: config},
			{
				PreConfig: func() {
					fake.mu.Lock()
					fake.adminInitialized = false
					fake.mu.Unlock()
				},
				Config:             config,
				PlanOnly:           true,
				ExpectNonEmptyPlan: true,
			},
		},
	})
}

// TestUserAdminResource_UpgradeFromV300: the bootstrap written by v3.0.0
// (SDKv2) plans no change with this build.
func TestUserAdminResource_UpgradeFromV300(t *testing.T) {
	t.Parallel()
	for name, body := range map[string]string{
		"defaults": `password = "pw-123456789"`,
		"all":      `username = "root"` + "\n" + `password = "pw-123456789"` + "\n" + `setup_token = "tok"`,
	} {
		t.Run(name, func(t *testing.T) {
			mock := NewMockServer(t)
			newFakeUsersAPI(mock)
			config := mockProviderConfig(mock) + "resource \"portainer_user_admin\" \"a\" {\n" + body + "\n}"
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
