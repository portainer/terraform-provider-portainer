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

// TestHelmRollbackResource_QueryFromArguments: every set argument goes into
// the query; zero values (recreate=false) are left out as SDKv2's GetOk did.
func TestHelmRollbackResource_QueryFromArguments(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("POST", "/endpoints/4/kubernetes/helm/my-release/rollback", RespondString(http.StatusOK, "application/json", `{}`))
	mock.On("POST", "/endpoints/1/kubernetes/helm/rel/rollback", RespondString(http.StatusOK, "application/json", `{}`))

	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{{
			Config: mockProviderConfig(mock) + `
resource "portainer_helm_rollback" "full" {
  endpoint_id  = 4
  release_name = "my-release"
  namespace    = "default"
  revision     = 2
  wait         = true
  force        = true
  recreate     = false
  timeout      = 120
}
resource "portainer_helm_rollback" "minimal" {
  endpoint_id  = 1
  release_name = "rel"
}
`,
			Check: resource.ComposeTestCheckFunc(
				resource.TestMatchResourceAttr("portainer_helm_rollback.full", "id", regexp.MustCompile(`^helm-rollback-4-my-release-\d+$`)),
				resource.TestCheckResourceAttr("portainer_helm_rollback.minimal", "recreate", "true"),
				resource.TestCheckResourceAttr("portainer_helm_rollback.minimal", "timeout", "300"),
				func(*terraform.State) error {
					full := mock.FindRequest("POST", "/endpoints/4/kubernetes/helm/my-release/rollback")
					if full == nil || full.Query != "namespace=default&revision=2&wait=true&force=true&timeout=120" {
						return fmt.Errorf("unexpected query for the full rollback: %+v", full)
					}
					minimal := mock.FindRequest("POST", "/endpoints/1/kubernetes/helm/rel/rollback")
					if minimal == nil || minimal.Query != "recreate=true&timeout=300" {
						return fmt.Errorf("unexpected query for the minimal rollback: %+v", minimal)
					}
					return nil
				},
			),
		}},
	})
}

func TestHelmRollbackResource_HTTPError(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("POST", "/endpoints/1/kubernetes/helm/rel/rollback", RespondString(http.StatusBadRequest, "application/json", `{"message":"no previous revision"}`))
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{{
			Config:      mockProviderConfig(mock) + `resource "portainer_helm_rollback" "r" {` + "\n" + `endpoint_id = 1` + "\n" + `release_name = "rel"` + "\n}",
			ExpectError: regexp.MustCompile(`failed to rollback Helm release rel`),
		}},
	})
}

func TestHelmRollbackResource_UpgradeFromV300(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("POST", "/endpoints/4/kubernetes/helm/my-release/rollback", RespondString(http.StatusOK, "application/json", `{}`))
	mock.On("POST", "/endpoints/1/kubernetes/helm/rel/rollback", RespondString(http.StatusOK, "application/json", `{}`))
	config := mockProviderConfig(mock) + `
resource "portainer_helm_rollback" "full" {
  endpoint_id   = 4
  release_name  = "my-release"
  namespace     = "default"
  revision      = 2
  wait          = true
  wait_for_jobs = true
  force         = true
  recreate      = false
  timeout       = 120
}
resource "portainer_helm_rollback" "minimal" {
  endpoint_id  = 1
  release_name = "rel"
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
				Check: func(*terraform.State) error {
					n := 0
					for _, r := range mock.Requests() {
						if strings.HasSuffix(r.Path, "/rollback") {
							n++
						}
					}
					if n != 2 {
						return fmt.Errorf("the upgrade must not roll back again, got %d rollbacks", n)
					}
					return nil
				},
			},
		},
	})
}
