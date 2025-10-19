resource "portainer_registry" "proget" {
  name     = var.proget_name
  url      = var.proget_url
  base_url = var.proget_base_url
  type     = 5
  username = var.proget_username
  password = var.proget_password
}
