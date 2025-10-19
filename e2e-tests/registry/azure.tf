resource "portainer_registry" "azure" {
  name     = var.azure_name
  url      = var.azure_url
  type     = 2
  username = var.azure_username
  password = var.azure_password
}
