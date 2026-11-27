resource "portainer_oidc_client_sessions_end" "this" {
  client_id = var.client_id
  user_id   = var.user_id

  triggers = {
    reason = var.reason
  }
}
