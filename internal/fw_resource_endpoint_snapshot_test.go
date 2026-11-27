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

// TestEndpointSnapshotResource_SingleAndAll: with endpoint_id the one
// environment is snapshotted and the ID is its identifier; without it every
// environment is, with the ID "all". Read and destroy never call Portainer.
func TestEndpointSnapshotResource_SingleAndAll(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("POST", "/endpoints/3/snapshot", RespondString(http.StatusNoContent, "", ""))
	mock.On("POST", "/endpoints/snapshot", RespondString(http.StatusNoContent, "", ""))

	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		CheckDestroy: func(_ *terraform.State) error {
			if n := len(mock.Requests()); n != 2 {
				return fmt.Errorf("expected only the two snapshot calls, got %d requests", n)
			}
			return nil
		},
		Steps: []resource.TestStep{{
			Config: mockProviderConfig(mock) + `
resource "portainer_endpoint_snapshot" "one" { endpoint_id = 3 }
resource "portainer_endpoint_snapshot" "all" {}
`,
			Check: resource.ComposeTestCheckFunc(
				resource.TestCheckResourceAttr("portainer_endpoint_snapshot.one", "id", "3"),
				resource.TestCheckResourceAttr("portainer_endpoint_snapshot.all", "id", "all"),
				resource.TestCheckNoResourceAttr("portainer_endpoint_snapshot.all", "endpoint_id"),
			),
		}},
	})
}

// TestEndpointSnapshotResource_HTTPError: anything other than 204 fails.
func TestEndpointSnapshotResource_HTTPError(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("POST", "/endpoints/9/snapshot", RespondString(http.StatusInternalServerError, "application/json", `{"message":"boom"}`))

	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{{
			Config:      mockProviderConfig(mock) + `resource "portainer_endpoint_snapshot" "s" { endpoint_id = 9 }`,
			ExpectError: regexp.MustCompile(`HTTP 500`),
		}},
	})
}

// TestEndpointSnapshotResource_UpgradeFromV300: state written by v3.0.0
// plans no change with this build, with and without endpoint_id.
func TestEndpointSnapshotResource_UpgradeFromV300(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("POST", "/endpoints/3/snapshot", RespondString(http.StatusNoContent, "", ""))
	mock.On("POST", "/endpoints/snapshot", RespondString(http.StatusNoContent, "", ""))
	config := mockProviderConfig(mock) + `
resource "portainer_endpoint_snapshot" "one" { endpoint_id = 3 }
resource "portainer_endpoint_snapshot" "all" {}
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

// TestAccEndpointSnapshotResource snapshots every environment of a plain CE
// instance.
func TestAccEndpointSnapshotResource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck: func() { accPreCheck(t) },
		Steps: []resource.TestStep{{
			ProtoV5ProviderFactories: testProtoV5Factories,
			Config:                   accProvider() + `resource "portainer_endpoint_snapshot" "all" {}`,
			Check:                    resource.TestCheckResourceAttr("portainer_endpoint_snapshot.all", "id", "all"),
		}},
	})
}
