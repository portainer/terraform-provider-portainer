package internal

import (
	"fmt"
	"net/http"
	"os"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

func registerServiceUpdateA2(mock *MockServer) {
	mock.On("GET", "/endpoints/2/docker/services", RespondJSON(http.StatusOK, []map[string]interface{}{
		{"ID": "svc-abc", "Spec": map[string]interface{}{"Name": "web"}},
		{"ID": "svc-def", "Spec": map[string]interface{}{"Name": "db"}},
	}))
	mock.On("PUT", "/endpoints/2/forceupdateservice", RespondJSON(http.StatusOK, map[string]interface{}{"Warnings": []string{"w"}}))
}

// TestEndpointServiceUpdateResource_HappyPath: the service name is resolved
// to its ID, the force update carries it with pullImage, and the ID is
// "<endpoint>-<service>". Read and destroy never call Portainer.
func TestEndpointServiceUpdateResource_HappyPath(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	registerServiceUpdateA2(mock)

	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		CheckDestroy: func(_ *terraform.State) error {
			if n := len(mock.Requests()); n != 2 {
				return fmt.Errorf("expected only the lookup and the update, got %d requests", n)
			}
			return nil
		},
		Steps: []resource.TestStep{{
			Config: mockProviderConfig(mock) + `resource "portainer_endpoint_service_update" "u" {
  endpoint_id  = 2
  service_name = "web"
  pull_image   = true
}`,
			Check: resource.ComposeTestCheckFunc(
				resource.TestCheckResourceAttr("portainer_endpoint_service_update.u", "id", "2-svc-abc"),
				func(_ *terraform.State) error {
					payload := decodeA2(t, mock.FindRequest("PUT", "/endpoints/2/forceupdateservice"))
					if payload["pullImage"] != true || payload["serviceID"] != "svc-abc" {
						return fmt.Errorf("unexpected payload %v", payload)
					}
					return nil
				},
			),
		}},
	})
}

func TestEndpointServiceUpdateResource_Errors(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct {
		setup func(*MockServer)
		want  string
	}{
		"service not found": {func(m *MockServer) {
			m.On("GET", "/endpoints/2/docker/services", RespondJSON(http.StatusOK, []map[string]interface{}{
				{"ID": "svc-1", "Spec": map[string]interface{}{"Name": "other"}},
			}))
		}, `service with name 'web' not found`},
		"list fails": {func(m *MockServer) {
			m.On("GET", "/endpoints/2/docker/services", RespondString(http.StatusInternalServerError, "application/json", `{"message":"boom"}`))
		}, `failed to fetch services`},
		"update fails": {func(m *MockServer) {
			m.On("GET", "/endpoints/2/docker/services", RespondJSON(http.StatusOK, []map[string]interface{}{
				{"ID": "svc-1", "Spec": map[string]interface{}{"Name": "web"}},
			}))
			m.On("PUT", "/endpoints/2/forceupdateservice", RespondString(http.StatusBadRequest, "application/json", `{"message":"nope"}`))
		}, `failed to update service`},
	} {
		t.Run(name, func(t *testing.T) {
			mock := NewMockServer(t)
			tc.setup(mock)
			resource.UnitTest(t, resource.TestCase{
				ProtoV5ProviderFactories: testProtoV5Factories,
				Steps: []resource.TestStep{{
					Config: mockProviderConfig(mock) + `resource "portainer_endpoint_service_update" "u" {
  endpoint_id  = 2
  service_name = "web"
}`,
					ExpectError: regexp.MustCompile(tc.want),
				}},
			})
			if name == "service not found" && mock.FindRequest("PUT", "/endpoints/2/forceupdateservice") != nil {
				t.Error("did not expect PUT when the service was not found")
			}
		})
	}
}

// TestEndpointServiceUpdateResource_UpgradeFromV300: state written by v3.0.0
// plans no change with this build, with pull_image set and defaulted.
func TestEndpointServiceUpdateResource_UpgradeFromV300(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	registerServiceUpdateA2(mock)
	config := mockProviderConfig(mock) + `
resource "portainer_endpoint_service_update" "pull" {
  endpoint_id  = 2
  service_name = "web"
  pull_image   = true
}
resource "portainer_endpoint_service_update" "plain" {
  endpoint_id  = 2
  service_name = "db"
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

// TestAccEndpointServiceUpdateResource needs a Swarm environment and a
// service in it.
func TestAccEndpointServiceUpdateResource(t *testing.T) {
	envID, service := os.Getenv("PORTAINER_ACC_SWARM_ENV_ID"), os.Getenv("PORTAINER_ACC_SWARM_SERVICE")
	if envID == "" || service == "" {
		t.Skip("PORTAINER_ACC_SWARM_ENV_ID and PORTAINER_ACC_SWARM_SERVICE must name a Swarm environment and a service in it")
	}
	resource.Test(t, resource.TestCase{
		PreCheck: func() { accPreCheck(t) },
		Steps: []resource.TestStep{{
			ProtoV5ProviderFactories: testProtoV5Factories,
			Config: accProvider() + fmt.Sprintf(`resource "portainer_endpoint_service_update" "u" {
  endpoint_id  = %s
  service_name = %q
}`, envID, service),
		}},
	})
}
