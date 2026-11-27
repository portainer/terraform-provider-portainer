package internal

import (
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// TestEndpointRegistriesDataSource_OmitsCredentials is the important one: the
// API hands back the full registry object including its password and access
// token, and neither may reach Terraform state.
func TestEndpointRegistriesDataSource_OmitsCredentials(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("GET", "/endpoints/1/registries", RespondJSON(http.StatusOK, []map[string]interface{}{{
		"Id": 4, "Name": "harbor", "URL": "registry.example.com", "BaseURL": "https://registry.example.com",
		"Type": 3, "Authentication": true, "Username": "robot",
		"Password": "super-secret", "AccessToken": "tok-secret",
	}}))

	name := "data.portainer_endpoint_registries.r"
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{{
			Config: mockProviderConfig(mock) + `data "portainer_endpoint_registries" "r" { environment_id = 1 }`,
			Check: resource.ComposeTestCheckFunc(
				resource.TestCheckResourceAttr(name, "id", "1/registries"),
				resource.TestCheckResourceAttr(name, "registries.0.name", "harbor"),
				resource.TestCheckResourceAttr(name, "registries.0.username", "robot"),
				resource.TestCheckResourceAttr(name, "registries.0.authentication", "true"),
				resource.TestCheckResourceAttr(name, "dockerhub_rate_limit", "0"),
				func(s *terraform.State) error {
					for k, v := range s.RootModule().Resources[name].Primary.Attributes {
						if strings.Contains(strings.ToLower(k), "password") || strings.Contains(strings.ToLower(k), "token") || strings.Contains(v, "secret") {
							return fmt.Errorf("registry credentials must never be mapped into state, found %s=%s", k, v)
						}
					}
					if mock.FindRequest("GET", "/endpoints/1/registries").Query != "" {
						return fmt.Errorf("no namespace was set, so none must be sent")
					}
					return nil
				},
			),
		}},
	})
}

// TestEndpointRegistriesDataSource_NamespaceAndDockerHub verifies the
// namespace filter and that the optional rate-limit lookup only happens when
// asked for.
func TestEndpointRegistriesDataSource_NamespaceAndDockerHub(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("GET", "/endpoints/1/registries", RespondJSON(http.StatusOK, []map[string]interface{}{}))
	mock.On("GET", "/endpoints/1/dockerhub/6", RespondJSON(http.StatusOK, map[string]interface{}{"limit": 200, "remaining": 47}))

	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{{
			Config: mockProviderConfig(mock) + `data "portainer_endpoint_registries" "r" {
  environment_id        = 1
  namespace             = "apps"
  dockerhub_registry_id = 6
}`,
			Check: resource.ComposeTestCheckFunc(
				resource.TestCheckResourceAttr("data.portainer_endpoint_registries.r", "dockerhub_rate_limit", "200"),
				resource.TestCheckResourceAttr("data.portainer_endpoint_registries.r", "dockerhub_rate_remaining", "47"),
				resource.TestCheckResourceAttr("data.portainer_endpoint_registries.r", "registries.#", "0"),
				func(_ *terraform.State) error {
					if q := mock.FindRequest("GET", "/endpoints/1/registries").Query; q != "namespace=apps" {
						return fmt.Errorf("expected the namespace filter, got %q", q)
					}
					return nil
				},
			),
		}},
	})
}
