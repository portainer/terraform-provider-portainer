# The parent must be a Kubernetes environment connected through an Edge Agent
# 3.0.0 or later in tunnel (standard) mode.
resource "portainer_d2k_environment" "team_a" {
  parent_environment_id = var.kubernetes_environment_id
  name                  = "team-a"
  environment_name      = "Team A (D2K)"
  swarm_mode            = false
}

# The D2K environment serves the Docker API, so Docker resources work on it.
resource "portainer_docker_network" "team_a" {
  endpoint_id = portainer_d2k_environment.team_a.environment_id
  name        = "team-a-net"
}

output "d2k_namespace" {
  value = portainer_d2k_environment.team_a.namespace
}
