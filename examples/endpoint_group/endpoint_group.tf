resource "portainer_endpoint_group" "your-group" {
  name        = var.portainer_endpoint_group_name
  description = var.portainer_endpoint_group_description
}