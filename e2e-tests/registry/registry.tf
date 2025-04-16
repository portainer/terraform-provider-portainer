resource "portainer_registry" "registry" {
  name           = var.portainer_registry_name
  type           = var.portainer_registry_type
  base_url       = var.portainer_registry_url
  url            = var.portainer_registry_url
  authentication = var.portainer_registry_authentication
}
