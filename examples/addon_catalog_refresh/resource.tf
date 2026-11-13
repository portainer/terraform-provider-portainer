resource "portainer_addon_catalog_refresh" "now" {
  triggers = {
    catalog_release = "2026-11"
  }
}
