data "portainer_docker_network" "example" {
  endpoint_id = var.endpoint_id
  name        = var.docker_network_name
}

output "docker_network_id" {
  value = data.portainer_docker_network.example.id
}

output "docker_network_driver" {
  value = data.portainer_docker_network.example.driver
}

output "docker_network_scope" {
  value = data.portainer_docker_network.example.scope
}
