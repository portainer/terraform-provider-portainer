data "portainer_oidc_client_sessions" "this" {
  client_id = var.client_id
}

output "signed_in_users" {
  value = distinct(data.portainer_oidc_client_sessions.this.sessions[*].username)
}
