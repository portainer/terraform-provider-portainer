resource "portainer_registry" "wo" {
  name                = "e2e-registry-password-wo"
  url                 = "registry.e2e.example.com"
  type                = 3
  authentication      = true
  username            = "e2e-bot"
  password_wo         = var.registry_password
  password_wo_version = var.registry_password_version
}
