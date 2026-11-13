# A D2K environment on k3d, and a Docker resource against it.
resource "portainer_d2k_environment" "e2e" {
  parent_environment_id = var.k8s_environment_id
  name                  = "e2e"
  environment_name      = "e2e-d2k"
  swarm_mode            = false
}

resource "portainer_docker_volume" "e2e" {
  endpoint_id = portainer_d2k_environment.e2e.environment_id
  name        = "e2e-d2k-volume"
}

output "environment_id" {
  value = portainer_d2k_environment.e2e.environment_id
}
