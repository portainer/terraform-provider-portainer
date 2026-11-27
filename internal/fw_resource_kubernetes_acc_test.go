package internal

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
)

// accKubernetesEnv skips an acceptance test without a Kubernetes environment
// to run against, and returns its ID.
func accKubernetesEnv(t *testing.T) string {
	t.Helper()
	accPreCheck(t)
	envID := os.Getenv("PORTAINER_ACC_K8S_ENV_ID")
	if envID == "" {
		t.Skip("PORTAINER_ACC_K8S_ENV_ID must name a Kubernetes environment for the Kubernetes acceptance tests")
	}
	return envID
}

// TestAccKubernetesNamespace_UpgradeFromSDKv2: a namespace and its system
// flag created by v3.0.0 (SDKv2) plan no change with this build, then import
// cleanly.
func TestAccKubernetesNamespace_UpgradeFromSDKv2(t *testing.T) {
	envID := accKubernetesEnv(t)
	name := acctest.RandomWithPrefix("acc-ns")
	// A licensed Business Edition takes requests and limits separately;
	// Community Edition one value for both.
	quota := `{ cpu = "1", memory = "256Mi" }`
	if accIsBE() {
		quota = `{ cpu_request = "500m", cpu_limit = "1", memory_request = "128Mi", memory_limit = "256Mi" }`
	}
	config := fmt.Sprintf(`
resource "portainer_kubernetes_namespace" "n" {
  environment_id = %[1]s
  name           = %[2]q
  annotations    = { team = "acc" }
  resource_quota = %[3]s
}

resource "portainer_kubernetes_namespace_system" "s" {
  environment_id = %[1]s
  namespace      = portainer_kubernetes_namespace.n.name
  system         = true
}
`, envID, name, quota)
	resource.Test(t, resource.TestCase{
		PreCheck: func() { accPreCheck(t) },
		Steps: []resource.TestStep{
			{
				ExternalProviders: previousRelease,
				Config:            accProvider() + config,
			},
			{
				ProtoV5ProviderFactories: testProtoV5Factories,
				Config:                   accProvider() + config,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
			{
				ProtoV5ProviderFactories: testProtoV5Factories,
				Config:                   accProvider() + config,
				ResourceName:             "portainer_kubernetes_namespace_system.s",
				ImportState:              true,
				ImportStateVerify:        true,
			},
		},
	})
}

// TestAccKubernetesEnvironmentConfig_Lifecycle: the configuration of a real
// Kubernetes environment is changed and read back; destroy only forgets it.
func TestAccKubernetesEnvironmentConfig_Lifecycle(t *testing.T) {
	envID := accKubernetesEnv(t)
	config := func(metrics bool) string {
		return accProvider() + fmt.Sprintf(`
resource "portainer_kubernetes_environment_config" "c" {
  endpoint_id        = %s
  use_server_metrics = %t
}
`, envID, metrics)
	}
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { accPreCheck(t) },
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{Config: config(true), Check: resource.TestCheckResourceAttr("portainer_kubernetes_environment_config.c", "use_server_metrics", "true")},
			{Config: config(false), Check: resource.TestCheckResourceAttr("portainer_kubernetes_environment_config.c", "use_server_metrics", "false")},
			// Import reads the environment's storage classes, which this
			// configuration leaves to Portainer.
			{ResourceName: "portainer_kubernetes_environment_config.c", ImportState: true, ImportStateVerify: true, ImportStateVerifyIgnore: []string{"storage_class"}},
		},
	})
}
