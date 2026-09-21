data "portainer_addons" "all" {}

output "addon_environment_id" {
  value = data.portainer_addons.all.environment_id
}

output "addons_with_upgrades" {
  value = [for a in data.portainer_addons.all.addons : a.id if a.upgrade_available]
}
