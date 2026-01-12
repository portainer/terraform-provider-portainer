# Custom Registry (Anonymous)
resource "portainer_registry" "custom" {
  name           = var.custom_name
  url            = var.custom_url
  type           = 3
  authentication = var.custom_authentication
}

# Custom Registry (Authentication)
resource "portainer_registry" "custom_auth" {
  name           = var.custom_auth_name
  url            = var.custom_auth_url
  type           = 3
  authentication = var.custom_auth_authentication
  username       = var.custom_auth_username
  password       = var.custom_auth_password
}

data "portainer_registry" "test_lookup" {
  name = portainer_registry.custom.name
}

output "found_registry_id" {
  value = data.portainer_registry.test_lookup.id
}
