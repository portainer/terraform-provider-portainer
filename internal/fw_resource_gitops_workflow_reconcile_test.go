package internal

import (
	"fmt"
	"net/http"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

func gitopsReconcileResConfig(m *MockServer, triggers string) string {
	return mockProviderConfig(m) + fmt.Sprintf(`
resource "portainer_gitops_workflow_reconcile" "r" {
  workflow_id = 4
  %s
}`, triggers)
}

// TestGitopsWorkflowReconcileResource_PostsAndRecordsRun: the action is one
// POST with no body recorded under a run ID, nothing is read back, and a
// trigger change asks for another reconcile.
func TestGitopsWorkflowReconcileResource_PostsAndRecordsRun(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("POST", "/gitops/workflows/4/reconcile", RespondString(http.StatusNoContent, "application/json", ""))

	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{
				Config: gitopsReconcileResConfig(mock, `triggers = { revision = "9f2c" }`),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrWith("portainer_gitops_workflow_reconcile.r", "id", func(id string) error {
						if !strings.HasPrefix(id, "4-reconcile-") {
							return fmt.Errorf("id: got %q", id)
						}
						return nil
					}),
					gitopsCheck(func() error {
						req := mock.FindRequest("POST", "/gitops/workflows/4/reconcile")
						if req == nil {
							return fmt.Errorf("expected POST /gitops/workflows/4/reconcile")
						}
						if len(req.Body) != 0 {
							return fmt.Errorf("the reconcile request has no body, got %q", req.Body)
						}
						return nil
					}),
				),
			},
			{
				Config:   gitopsReconcileResConfig(mock, `triggers = { revision = "9f2c" }`),
				PlanOnly: true,
			},
			{
				Config: gitopsReconcileResConfig(mock, `triggers = { revision = "a71b" }`),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction("portainer_gitops_workflow_reconcile.r", plancheck.ResourceActionDestroyBeforeCreate)},
				},
				Check: resource.ComposeTestCheckFunc(
					func(*terraform.State) error {
						if n := gitopsRequestCount(mock, "POST", "/gitops/workflows/4/reconcile"); n != 2 {
							return fmt.Errorf("expected a second reconcile, got %d requests", n)
						}
						return nil
					},
				),
			},
		},
	})
}

// TestGitopsWorkflowReconcileResource_OlderServerNamesRequirement: Portainer
// 2.45 has no reconcile endpoint, and its 404 has to say why rather than
// "not found"; nothing is recorded.
func TestGitopsWorkflowReconcileResource_OlderServerNamesRequirement(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("POST", "/gitops/workflows/4/reconcile", RespondString(http.StatusNotFound, "text/plain", "404 page not found"))
	mock.On("GET", "/system/version", RespondJSON(http.StatusOK, map[string]interface{}{
		"ServerVersion": "2.45.3", "ServerEdition": "BE",
	}))

	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{{
			Config:      gitopsReconcileResConfig(mock, ""),
			ExpectError: regexp.MustCompile(`requires Portainer Business Edition 3.0 or later`),
		}},
	})
}

// TestGitopsWorkflowReconcile_UpgradeFromV300: a reconcile recorded by v3.0.0
// (SDKv2) is not asked for again by this build.
func TestGitopsWorkflowReconcile_UpgradeFromV300(t *testing.T) {
	t.Parallel()
	for name, triggers := range map[string]string{
		"without triggers": "",
		"with triggers":    `triggers = { revision = "9f2c" }`,
	} {
		t.Run(name, func(t *testing.T) {
			mock := NewMockServer(t)
			mock.On("POST", "/gitops/workflows/4/reconcile", RespondString(http.StatusNoContent, "application/json", ""))
			config := gitopsReconcileResConfig(mock, triggers)
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

// TestAccGitopsWorkflowReconcileResource needs a Business Edition 3.0
// instance with a workflow on the reconcile engine.
func TestAccGitopsWorkflowReconcileResource(t *testing.T) {
	if os.Getenv("PORTAINER_ACC_BE") != "1" || os.Getenv("PORTAINER_ACC_RECONCILE_WORKFLOW_ID") == "" {
		t.Skip("set PORTAINER_ACC_BE=1 and PORTAINER_ACC_RECONCILE_WORKFLOW_ID (a workflow on the reconcile engine) to run against Business Edition 3.0")
	}
	config := accProvider() + fmt.Sprintf(`
resource "portainer_gitops_workflow_reconcile" "r" {
  workflow_id = %s
  triggers    = { run = "1" }
}`, os.Getenv("PORTAINER_ACC_RECONCILE_WORKFLOW_ID"))
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { accPreCheck(t) },
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{Config: config, Check: resource.TestCheckResourceAttrSet("portainer_gitops_workflow_reconcile.r", "id")},
			{Config: config, PlanOnly: true},
		},
	})
}
