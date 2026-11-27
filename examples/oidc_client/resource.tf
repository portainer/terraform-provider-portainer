resource "portainer_settings" "this" {
  oidc_provider {
    public_url            = var.portainer_public_url
    access_token_lifetime = "15m"
  }
}

resource "portainer_team" "ops" {
  name = "ops"
}

resource "portainer_oidc_client" "grafana" {
  name             = "Grafana"
  description      = "Dashboards"
  redirect_uris    = ["${var.grafana_url}/login/generic_oauth"]
  homepage_url     = var.grafana_url
  show_in_launcher = true
  groups_claim     = true

  team_access_policies = {
    (portainer_team.ops.id) = 1
  }

  depends_on = [portainer_settings.this]
}

output "oidc_issuer" {
  value = "${var.portainer_public_url}/api/oidc"
}

output "grafana_oauth_client_id" {
  value = portainer_oidc_client.grafana.client_id
}

output "grafana_oauth_client_secret" {
  value     = portainer_oidc_client.grafana.client_secret
  sensitive = true
}
