resource "portainer_user_admin" "bootstrap" {
  username    = "admin"
  password    = "e2e-Bootstrap-Admin-123"
  setup_token = var.setup_token
}
