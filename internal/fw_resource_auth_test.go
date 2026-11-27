package internal

import (
	"fmt"
	"net/http"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

const authConfig = `
resource "portainer_auth" "a" {
  username = "admin"
  password = "s3cret"
}`

// TestAuthResource_IssuesJWT: the credentials are posted to /auth and the
// JWT lands in state.
func TestAuthResource_IssuesJWT(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("POST", "/auth", RespondJSON(http.StatusOK, map[string]interface{}{"jwt": "eyJ.token"}))
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{
				Config: mockProviderConfig(mock) + authConfig,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("portainer_auth.a", "id", "auth-result"),
					resource.TestCheckResourceAttr("portainer_auth.a", "jwt", "eyJ.token"),
					func(*terraform.State) error {
						body := gsbBody(t, mock, "POST", "/auth")
						if body["username"] != "admin" || body["password"] != "s3cret" {
							return fmt.Errorf("payload mismatch: %v", body)
						}
						return nil
					},
				),
			},
			{Config: mockProviderConfig(mock) + authConfig, PlanOnly: true},
		},
	})
}

func TestAuthResource_HTTPError(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("POST", "/auth", RespondString(http.StatusUnauthorized, "application/json", `{"message":"invalid credentials"}`))
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{{
			Config:      mockProviderConfig(mock) + authConfig,
			ExpectError: regexp.MustCompile(`failed to authenticate`),
		}},
	})
}

func TestAuthResource_UpgradeFromV300(t *testing.T) {
	t.Parallel()
	newMock := func(t *testing.T) *MockServer {
		m := NewMockServer(t)
		m.On("POST", "/auth", RespondJSON(http.StatusOK, map[string]interface{}{"jwt": "eyJ.token"}))
		return m
	}
	gsbUpgradeTest(t, newMock, authConfig)
}
