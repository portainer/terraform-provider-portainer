resource "portainer_addon_repair" "portal" {
  addon_id = "portal-template"

  triggers = {
    # Repair again whenever the addon reports a broken credential.
    health = "credential-invalid"
  }
}

output "addon_repair_status" {
  value = portainer_addon_repair.portal.lifecycle_status
}
