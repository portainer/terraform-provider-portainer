# Write-only: the license key never reaches the Terraform state.
resource "portainer_licenses" "be" {
  key_wo         = var.license_key
  key_wo_version = 1
}
