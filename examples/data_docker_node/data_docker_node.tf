data "portainer_docker_node" "example" {
  endpoint_id = var.endpoint_id
  hostname    = var.docker_node_hostname
}

output "docker_node_id" {
  value = data.portainer_docker_node.example.id
}

output "docker_node_role" {
  value = data.portainer_docker_node.example.role
}

output "docker_node_status" {
  value = data.portainer_docker_node.example.status
}
