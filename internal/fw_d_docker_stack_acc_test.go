package internal

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// Acceptance tests of the Docker and stack resources against a real
// Portainer. A Docker standalone environment is named by
// PORTAINER_ACC_DOCKER_ENV_ID, a Swarm one by PORTAINER_ACC_SWARM_ENV_ID.

func dkAccEnv(t *testing.T, variable string) string {
	t.Helper()
	accPreCheck(t)
	id := os.Getenv(variable)
	if id == "" {
		t.Skipf("%s must name a Portainer environment for this acceptance test", variable)
	}
	return id
}

// dkAccUpgradeSteps applies config with v3.0.0, plans it with this build
// (which has to plan nothing) and imports address with importID.
func dkAccUpgradeSteps(config, address string, importID resource.ImportStateIdFunc, ignore ...string) []resource.TestStep {
	return []resource.TestStep{
		{ExternalProviders: previousRelease, Config: accProvider() + config},
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
			ResourceName:             address,
			ImportState:              true,
			ImportStateIdFunc:        importID,
			ImportStateVerify:        true,
			ImportStateVerifyIgnore:  ignore,
		},
	}
}

func TestAccStackResource_UpgradeFromSDKv2(t *testing.T) {
	env := dkAccEnv(t, "PORTAINER_ACC_DOCKER_ENV_ID")
	name := acctest.RandomWithPrefix("acc-stack")
	config := fmt.Sprintf(`
resource "portainer_stack" "s" {
  name               = %q
  deployment_type    = "standalone"
  method             = "string"
  endpoint_id        = %s
  stack_file_content = "services:\n  web:\n    image: nginx:alpine\n"
  env {
    name  = "A"
    value = "1"
  }
}

data "portainer_stack" "s" {
  name        = portainer_stack.s.name
  endpoint_id = portainer_stack.s.endpoint_id
}`, name, env)
	resource.Test(t, resource.TestCase{
		PreCheck: func() { accPreCheck(t) },
		Steps: dkAccUpgradeSteps(config, "portainer_stack.s", func(s *terraform.State) (string, error) {
			return fmt.Sprintf("%s:%s:standalone:string", env, s.RootModule().Resources["portainer_stack.s"].Primary.ID), nil
		}, "file_path_in_repository"),
	})
}

// TestAccDockerObjects_UpgradeFromSDKv2 leaves portainer_docker_network out:
// v3.0.0 panics reading a network whose IPAM config has a gateway ("Invalid
// address to set: ipam_config.0.Gateway"), which Docker reports for every
// network it assigns a subnet. TestAccDockerNetworkResource covers it.
func TestAccDockerObjects_UpgradeFromSDKv2(t *testing.T) {
	env := dkAccEnv(t, "PORTAINER_ACC_DOCKER_ENV_ID")
	name := acctest.RandomWithPrefix("acc-docker")
	config := fmt.Sprintf(`
resource "portainer_docker_volume" "v" {
  endpoint_id = %[1]s
  name        = %[2]q
}

resource "portainer_docker_image" "i" {
  endpoint_id = %[1]s
  image       = "busybox:latest"
}`, env, name)
	resource.Test(t, resource.TestCase{
		PreCheck: func() { accPreCheck(t) },
		Steps: dkAccUpgradeSteps(config, "portainer_docker_volume.v", func(*terraform.State) (string, error) {
			return env + ":" + name, nil
		}),
	})
}

// TestAccDockerNetworkResource creates a network with this build (v3.0.0
// cannot read one back, see above), plans nothing on a second run and reads
// it through the data source.
func TestAccDockerNetworkResource(t *testing.T) {
	env := dkAccEnv(t, "PORTAINER_ACC_DOCKER_ENV_ID")
	name := acctest.RandomWithPrefix("acc-network")
	config := accProvider() + fmt.Sprintf(`
resource "portainer_docker_network" "n" {
  endpoint_id = %[1]s
  name        = %[2]q
  labels      = { owner = "acc" }
}

data "portainer_docker_network" "n" {
  endpoint_id = %[1]s
  name        = portainer_docker_network.n.name
}`, env, name)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { accPreCheck(t) },
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{
				Config: config,
				Check:  resource.TestCheckResourceAttr("data.portainer_docker_network.n", "name", name),
			},
			{
				Config: config,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
		},
	})
}

func TestAccDockerSwarmObjects_UpgradeFromSDKv2(t *testing.T) {
	env := dkAccEnv(t, "PORTAINER_ACC_SWARM_ENV_ID")
	name := acctest.RandomWithPrefix("acc-swarm")
	config := fmt.Sprintf(`
resource "portainer_docker_secret" "s" {
  endpoint_id = %[1]s
  name        = %[2]q
  data        = "c2VjcmV0"
}

resource "portainer_docker_config" "c" {
  endpoint_id = %[1]s
  name        = %[2]q
  data        = "Y29uZmln"
}`, env, name)
	resource.Test(t, resource.TestCase{
		PreCheck: func() { accPreCheck(t) },
		Steps: []resource.TestStep{
			{ExternalProviders: previousRelease, Config: accProvider() + config},
			{
				ProtoV5ProviderFactories: testProtoV5Factories,
				Config:                   accProvider() + config,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
		},
	})
}
