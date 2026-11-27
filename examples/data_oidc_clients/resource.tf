data "portainer_oidc_clients" "by_name" {
  name = var.client_name
}

output "oauth_client_ids" {
  value = data.portainer_oidc_clients.by_name.clients[*].client_id
}

output "secret_ids" {
  value = flatten(data.portainer_oidc_clients.by_name.clients[*].secrets[*].id)
}
