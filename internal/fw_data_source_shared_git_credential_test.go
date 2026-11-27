package internal

import (
	"net/http"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestSharedGitCredentialDataSource(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("GET", "/cloud/gitcredentials", RespondJSON(http.StatusOK, []map[string]interface{}{
		{"id": 1, "userId": 10, "name": "ci_token", "username": "robot", "authorizationType": 1},
		{"id": 2, "userId": 20, "name": "ops_token", "username": "ops", "authorizationType": 0},
	}))
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{
				Config: mockProviderConfig(mock) + `data "portainer_shared_git_credential" "g" { name = "ops_token" }`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("data.portainer_shared_git_credential.g", "id", "2"),
					resource.TestCheckResourceAttr("data.portainer_shared_git_credential.g", "username", "ops"),
					resource.TestCheckResourceAttr("data.portainer_shared_git_credential.g", "authorization_type", "0"),
					resource.TestCheckResourceAttr("data.portainer_shared_git_credential.g", "user_id", "20"),
				),
			},
			{
				Config:      mockProviderConfig(mock) + `data "portainer_shared_git_credential" "g" { name = "ghost" }`,
				ExpectError: regexp.MustCompile(`shared git credential with name "ghost" not found`),
			},
		},
	})
}

func TestSharedGitCredentialDataSource_HTTPError(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("GET", "/cloud/gitcredentials", RespondString(http.StatusInternalServerError, "application/json", `{"message":"boom"}`))
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{{
			Config:      mockProviderConfig(mock) + `data "portainer_shared_git_credential" "g" { name = "ci_token" }`,
			ExpectError: regexp.MustCompile(`failed to list shared git credentials`),
		}},
	})
}
