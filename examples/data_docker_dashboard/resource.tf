data "portainer_docker_dashboard" "prod" {
  environment_id = var.environment_id
}

output "unhealthy_containers" {
  value = data.portainer_docker_dashboard.prod.containers_unhealthy
}
