data "portainer_addon_catalog" "catalog" {}

output "addon_charts" {
  value = { for a in data.portainer_addon_catalog.catalog.addons : a.id => a.helm_chart_repository }
}
