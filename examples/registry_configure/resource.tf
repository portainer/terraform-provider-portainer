resource "portainer_registry_configure" "harbor" {
  registry_id    = 4
  authentication = true
  username       = "robot$ci"
  password       = var.secret
  tls            = true
}
