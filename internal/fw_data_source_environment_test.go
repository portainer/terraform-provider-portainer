package internal

import (
	"net/http"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestEnvironmentDataSource(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("GET", "/endpoints", RespondJSON(http.StatusOK, []map[string]interface{}{
		{"Id": 1, "Name": "dev", "Type": 1, "URL": "tcp://dev:2375", "GroupId": 1},
		{"Id": 5, "Name": "prod", "Type": 2, "URL": "tcp://prod:2375", "GroupId": 3},
	}))
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{
				Config: mockProviderConfig(mock) + `data "portainer_environment" "e" { name = "prod" }`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("data.portainer_environment.e", "id", "5"),
					resource.TestCheckResourceAttr("data.portainer_environment.e", "type", "2"),
					resource.TestCheckResourceAttr("data.portainer_environment.e", "environment_address", "tcp://prod:2375"),
					resource.TestCheckResourceAttr("data.portainer_environment.e", "group_id", "3"),
				),
			},
			{
				Config:      mockProviderConfig(mock) + `data "portainer_environment" "e" { name = "missing" }`,
				ExpectError: regexp.MustCompile(`environment missing not found`),
			},
		},
	})
}

func TestEnvironmentDataSource_HTTPError(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("GET", "/endpoints", RespondString(http.StatusInternalServerError, "application/json", `{"message":"boom"}`))
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{{
			Config:      mockProviderConfig(mock) + `data "portainer_environment" "e" { name = "prod" }`,
			ExpectError: regexp.MustCompile(`failed to list environments`),
		}},
	})
}
