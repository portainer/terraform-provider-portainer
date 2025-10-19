# GitHub (official BE-supported)
resource "portainer_registry" "github" {
  name                     = var.github_name
  type                     = 8
  url                      = var.github_url
  authentication           = var.github_authentication
  username                 = var.github_username
  password                 = var.github_password
  github_use_organisation  = var.github_use_organisation
  github_organisation_name = var.github_organisation_name
}

# GitHub (CE workaround - custom)
resource "portainer_registry" "github_custom" {
  name           = var.github_custom_name
  url            = var.github_custom_url
  type           = 3
  authentication = var.github_custom_authentication
  username       = var.github_custom_username
  password       = var.github_custom_password
}
