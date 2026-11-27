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

// sessionProviderConfig logs the provider in with a user and password, as
// portainer_user_api_key requires.
func sessionProviderConfig(m *MockServer, user, password string) string {
	return fmt.Sprintf(`
provider "portainer" {
  endpoint     = %q
  api_user     = %q
  api_password = %q
  max_retries  = 0
}
`, m.URL, user, password)
}

// TestUserAPIKeyResource_Lifecycle: the key is created against the user's
// token endpoint, the raw key (returned once) is kept, the plan settles, a new
// description replaces the key and destroy revokes it.
func TestUserAPIKeyResource_Lifecycle(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	fake := newFakeUsersAPI(mock)
	user := fake.addUser("admin", "pw-1234567890", 1)
	cfg := func(desc string) string {
		return sessionProviderConfig(mock, "admin", "pw-1234567890") + fmt.Sprintf(`resource "portainer_user_api_key" "k" {
  user_id     = %d
  description = %q
  password    = "pw-1234567890"
}`, user, desc)
	}
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{
				Config: cfg("ci"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("portainer_user_api_key.k", "id", "2"),
					resource.TestCheckResourceAttr("portainer_user_api_key.k", "raw_api_key", "ptr_raw_2"),
					resource.TestCheckResourceAttr("portainer_user_api_key.k", "prefix", "ptr_abc"),
					resource.TestCheckResourceAttr("portainer_user_api_key.k", "date_created", "1700000000"),
					resource.TestCheckResourceAttr("portainer_user_api_key.k", "last_used", "0"),
					func(*terraform.State) error {
						var body map[string]interface{}
						if err := mock.FindRequest("POST", "/users/1/tokens").DecodeJSON(&body); err != nil || body["description"] != "ci" || body["password"] != "pw-1234567890" {
							return fmt.Errorf("unexpected create payload %v (%w)", body, err)
						}
						return nil
					},
				),
			},
			{
				PreConfig: func() {
					fake.mu.Lock()
					fake.keys[2].LastUsed = 1700000500
					fake.mu.Unlock()
				},
				Config: cfg("ci"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
				Check: resource.TestCheckResourceAttr("portainer_user_api_key.k", "last_used", "1700000500"),
			},
			{
				Config: cfg("deploy"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction("portainer_user_api_key.k", plancheck.ResourceActionDestroyBeforeCreate)},
				},
				Check: func(*terraform.State) error {
					if mock.FindRequest("DELETE", "/users/1/tokens/2") == nil {
						return fmt.Errorf("expected the old key revoked")
					}
					return nil
				},
			},
		},
	})
	if len(fake.keys) != 0 {
		t.Error("expected every key revoked on destroy")
	}
}

// TestUserAPIKeyResource_RejectsAPIKeyAuth: Portainer accepts the call only
// from a session, so the provider refuses before sending.
func TestUserAPIKeyResource_RejectsAPIKeyAuth(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	newFakeUsersAPI(mock)
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{{
			Config: mockProviderConfig(mock) + `resource "portainer_user_api_key" "k" {
  user_id = 1
  description = "x"
  password = "pw"
}`,
			ExpectError: regexp.MustCompile(`only accepts this call from a\s+session`),
		}},
	})
	if mock.FindRequest("POST", "/users/1/tokens") != nil {
		t.Error("expected no request with API key authentication")
	}
}

// TestUserAPIKeyResource_NoIDReturned: a response without a key ID fails
// instead of leaving a resource that can never be revoked.
func TestUserAPIKeyResource_NoIDReturned(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("POST", "/users/1/tokens", RespondJSON(http.StatusCreated, map[string]interface{}{"rawAPIKey": "ptr_x", "apiKey": map[string]interface{}{}}))
	fake := newFakeUsersAPI(mock)
	fake.addUser("admin", "pw", 1)
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{{
			Config: sessionProviderConfig(mock, "admin", "pw") + `resource "portainer_user_api_key" "k" {
  user_id = 1
  description = "x"
  password = "pw"
}`,
			ExpectError: regexp.MustCompile(`Portainer returned no key ID`),
		}},
	})
}

// TestUserAPIKeyResource_RevokedElsewhere: a key revoked outside Terraform
// drops out of state, so it is planned again; a key whose user is gone too.
func TestUserAPIKeyResource_RevokedElsewhere(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	fake := newFakeUsersAPI(mock)
	fake.addUser("admin", "pw", 1)
	config := sessionProviderConfig(mock, "admin", "pw") + `resource "portainer_user_api_key" "k" {
  user_id = 1
  description = "x"
  password = "pw"
}`
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{Config: config},
			{
				PreConfig: func() {
					fake.mu.Lock()
					delete(fake.keys, 2)
					fake.mu.Unlock()
				},
				Config:             config,
				PlanOnly:           true,
				ExpectNonEmptyPlan: true,
			},
		},
	})
}

func TestUserAPIKeyResource_UpgradeFromV300(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	fake := newFakeUsersAPI(mock)
	fake.addUser("admin", "pw-1234567890", 1)
	config := sessionProviderConfig(mock, "admin", "pw-1234567890") + `resource "portainer_user_api_key" "k" {
  user_id     = 1
  description = "ci"
  password    = "pw-1234567890"
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
