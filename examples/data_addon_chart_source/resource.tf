data "portainer_addon_chart_source" "portal" {
  addon_id = "portal-template"
}

output "addon_chart_reachable" {
  value = data.portainer_addon_chart_source.portal.reachable
}

output "addon_chart_versions" {
  value = data.portainer_addon_chart_source.portal.versions
}
