package internal

import (
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

func omniNodeRebootHCL(triggers string) string {
	return fmt.Sprintf(`
resource "portainer_omni_node_reboot" "r" {
  credential_id = 3
  cluster       = "edge-fleet"
  node          = "cp-1"
  %s
}
`, triggers)
}

func omniRebootCount(mock *MockServer) int {
	n := 0
	for _, r := range mock.Requests() {
		if r.Method == http.MethodPost && r.Path == "/omni/3/node/cp-1" {
			n++
		}
	}
	return n
}

// TestOmniNodeReboot_PassesClusterAsQuery pins the reboot action and its ID,
// then that the resource stays put until a trigger changes.
func TestOmniNodeReboot_PassesClusterAsQuery(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("POST", "/omni/3/node/cp-1", RespondJSON(http.StatusOK, map[string]interface{}{}))

	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{
				Config: mockProviderConfig(mock) + omniNodeRebootHCL(`triggers = { run = "1" }`),
				Check: resource.ComposeTestCheckFunc(
					resource.TestMatchResourceAttr("portainer_omni_node_reboot.r", "id", regexp.MustCompile(`^3/edge-fleet/cp-1/reboot-\d+$`)),
					func(*terraform.State) error {
						req := mock.FindRequest("POST", "/omni/3/node/cp-1")
						if req == nil || !strings.Contains(req.Query, "cluster=edge-fleet") {
							return fmt.Errorf("expected the cluster name in the query, got %+v", req)
						}
						return nil
					},
				),
			},
			{
				Config: mockProviderConfig(mock) + omniNodeRebootHCL(`triggers = { run = "1" }`),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
			{
				Config: mockProviderConfig(mock) + omniNodeRebootHCL(`triggers = { run = "2" }`),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction("portainer_omni_node_reboot.r", plancheck.ResourceActionReplace)},
				},
				Check: func(*terraform.State) error {
					if n := omniRebootCount(mock); n != 2 {
						return fmt.Errorf("a changed trigger must reboot again, got %d reboots", n)
					}
					return nil
				},
			},
		},
	})
}

func TestOmniNodeReboot_ErrorIsReported(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("POST", "/omni/3/node/cp-1", RespondString(http.StatusInternalServerError, "application/json", `{"message":"boom"}`))

	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{{
			Config:      mockProviderConfig(mock) + omniNodeRebootHCL(""),
			ExpectError: regexp.MustCompile(`failed to reboot node cp-1 of Omni cluster edge-fleet`),
		}},
	})
}

// TestOmniNodeReboot_UpgradeFromV300: reboots recorded by v3.0.0 (SDKv2), with
// and without triggers, plan no change - and so do not reboot again.
func TestOmniNodeReboot_UpgradeFromV300(t *testing.T) {
	t.Parallel()
	for name, triggers := range map[string]string{
		"with triggers":    `triggers = { run = "1" }`,
		"without triggers": "",
	} {
		t.Run(name, func(t *testing.T) {
			mock := NewMockServer(t)
			mock.On("POST", "/omni/3/node/cp-1", RespondJSON(http.StatusOK, map[string]interface{}{}))
			config := mockProviderConfig(mock) + omniNodeRebootHCL(triggers)

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
			if n := omniRebootCount(mock); n != 1 {
				t.Errorf("the upgrade must not reboot the node again, got %d reboots", n)
			}
		})
	}
}
