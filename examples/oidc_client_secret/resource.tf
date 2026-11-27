resource "portainer_oidc_client" "app" {
  name          = "internal-portal"
  redirect_uris = ["https://portal.example.com/oauth/callback"]
  all_users     = true
}

resource "portainer_oidc_client_secret" "app" {
  client_id   = tonumber(portainer_oidc_client.app.id)
  description = "production"

  regenerate_version      = var.secret_generation
  old_secret_grace_period = "24h"
}

output "client_id" {
  value = portainer_oidc_client.app.client_id
}

output "client_secret" {
  value     = portainer_oidc_client_secret.app.value
  sensitive = true
}
