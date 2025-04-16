resource "portainer_registry" "registry" {
  name     = var.portainer_registry_name
  type     = var.portainer_registry_type
  url      = var.portainer_registry_url
  username = var.portainer_registry_username
  password = var.portainer_registry_password
}
