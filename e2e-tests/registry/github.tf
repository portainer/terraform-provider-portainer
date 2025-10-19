# GitHub (CE workaround - custom)
resource "portainer_registry" "github_custom" {
  name           = var.github_custom_name
  url            = var.github_custom_url
  type           = 3
  authentication = var.github_custom_authentication
  username       = var.github_custom_username
  password       = var.github_custom_password
}
