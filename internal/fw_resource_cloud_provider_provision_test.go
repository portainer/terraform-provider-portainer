package internal

import (
	"fmt"
	"net/http"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

func provisionConfig(provider string) string {
	return fmt.Sprintf(`
resource "portainer_cloud_provider_provision" "p" {
  cloud_provider = %q
  payload = {
    name   = "ci-cluster"
    region = "FRA1"
  }
}`, provider)
}

// TestCloudProvisionResource_Civo: the payload map is posted to
// /cloud/{provider}/provision and the returned Id becomes the ID.
func TestCloudProvisionResource_Civo(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("POST", "/cloud/civo/provision", RespondJSON(http.StatusOK, map[string]interface{}{"Id": 27}))
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{{
			Config: mockProviderConfig(mock) + provisionConfig("civo"),
			Check: resource.ComposeTestCheckFunc(
				resource.TestCheckResourceAttr("portainer_cloud_provider_provision.p", "id", "27"),
				func(*terraform.State) error {
					req := mock.FindRequest("POST", "/cloud/civo/provision")
					if got := req.Headers.Get("X-API-Key"); got != "test-api-key" {
						return fmt.Errorf("X-API-Key header: got %q", got)
					}
					body := gsbBody(t, mock, "POST", "/cloud/civo/provision")
					if body["name"] != "ci-cluster" || body["region"] != "FRA1" {
						return fmt.Errorf("payload mismatch: %v", body)
					}
					return nil
				},
			),
		}},
	})
}

// TestCloudProvisionResource_RoutesByProvider: the URL follows cloud_provider.
func TestCloudProvisionResource_RoutesByProvider(t *testing.T) {
	t.Parallel()
	for _, prov := range []string{"digitalocean", "linode", "amazon", "azure", "gke"} {
		t.Run(prov, func(t *testing.T) {
			mock := NewMockServer(t)
			mock.On("POST", "/cloud/"+prov+"/provision", RespondJSON(http.StatusOK, map[string]interface{}{"Id": 1}))
			resource.UnitTest(t, resource.TestCase{
				ProtoV5ProviderFactories: testProtoV5Factories,
				Steps: []resource.TestStep{{
					Config: mockProviderConfig(mock) + provisionConfig(prov),
					Check:  resource.TestCheckResourceAttr("portainer_cloud_provider_provision.p", "id", "1"),
				}},
			})
		})
	}
}

func TestCloudProvisionResource_Errors(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("POST", "/cloud/civo/provision", RespondString(http.StatusBadRequest, "application/json", `{"message":"bad region"}`))
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{Config: mockProviderConfig(mock) + provisionConfig("civo"), ExpectError: regexp.MustCompile(`cloud provision failed`)},
			{Config: mockProviderConfig(mock) + provisionConfig("vultr"), ExpectError: regexp.MustCompile(`value must be one of`)},
		},
	})
}

func TestCloudProvisionResource_UpgradeFromV300(t *testing.T) {
	t.Parallel()
	newMock := func(t *testing.T) *MockServer {
		m := NewMockServer(t)
		m.On("POST", "/cloud/civo/provision", RespondJSON(http.StatusOK, map[string]interface{}{"Id": 27}))
		return m
	}
	gsbUpgradeTest(t, newMock,
		provisionConfig("civo"),
		`
resource "portainer_cloud_provider_provision" "p" {
  cloud_provider = "civo"
  payload        = { name = "x" }
  timeouts {
    create = "45m"
  }
}`,
	)
}
