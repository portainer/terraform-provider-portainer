resource "portainer_addon_config" "portal" {
  addon_id = "portal-template"

  entry {
    key   = "BASE_DOMAIN"
    value = "apps.example.com"
  }

  entry {
    key       = "API_TOKEN"
    value     = var.secret
    sensitive = true
  }
}
