resource "portainer_addon" "portal" {
  addon_id = "portal-template"
  version  = "1.2.3"

  values = jsonencode({
    ingress = {
      enabled = true
      host    = "apps.example.com"
    }
    replicas = 2
  })
}

output "addon_chart_version" {
  value = portainer_addon.portal.chart_version
}

output "addon_upgrade_available" {
  value = portainer_addon.portal.upgrade_available
}
