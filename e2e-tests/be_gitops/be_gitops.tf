# An Edge stack workflow as on 2.45, which has to behave the same on 3.0.
resource "portainer_edge_group" "e2e" {
  name    = "e2e-gitops"
  dynamic = false
}

resource "portainer_gitops_source" "e2e" {
  url      = "https://github.com/portainer/terraform-provider-portainer"
  name     = "e2e-gitops"
  interval = "1h"
  public   = true
}

resource "portainer_gitops_workflow" "e2e" {
  name       = "e2e-gitops"
  on_destroy = "destroy"

  artifact {
    name            = "e2e-agent"
    type            = "edgeStack"
    deployment_type = "compose"
    edge_group_ids  = [tonumber(portainer_edge_group.e2e.id)]

    file {
      source_id = tonumber(portainer_gitops_source.e2e.id)
      path      = "docker-compose.agent.yml"
      ref       = "refs/heads/main"
    }
  }
}
