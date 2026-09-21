data "portainer_licenses_info" "current" {}

output "licence_valid" {
  value = data.portainer_licenses_info.current.valid
}

output "licence_overused" {
  value = data.portainer_licenses_info.current.overuse_started_at > 0
}
