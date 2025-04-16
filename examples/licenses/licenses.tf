resource "portainer_licenses" "example" {
  key   = var.license_key
  force = var.license_force
}
