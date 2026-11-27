resource "portainer_oidc_user_sessions_end" "this" {
  user_id = var.user_id

  triggers = {
    reason = var.reason
  }
}
