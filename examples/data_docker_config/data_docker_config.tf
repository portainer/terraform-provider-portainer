data "portainer_docker_config" "example" {
  endpoint_id = var.endpoint_id
  name        = var.docker_config_name
}

output "docker_config_id" {
  value = data.portainer_docker_config.example.id
}

output "docker_config_name" {
  value = data.portainer_docker_config.example.name
}
