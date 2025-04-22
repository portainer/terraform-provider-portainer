resource "portainer_docker_node" "example" {
  endpoint_id  = var.docker_node_endpoint_id
  node_id      = var.docker_node_id
  version      = var.docker_node_version
  name         = var.docker_node_name
  availability = var.docker_node_availability
  role         = var.docker_node_role
  labels       = var.docker_node_labels
}
