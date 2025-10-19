resource "portainer_registry" "quay" {
  name                   = var.quay_name
  url                    = var.quay_url
  type                   = 1
  authentication         = var.quay_authentication
  username               = var.quay_username
  password               = var.quay_password
  quay_use_organisation  = var.quay_use_organisation
  quay_organisation_name = var.quay_organisation_name
}
