resource "portainer_user_admin" "init_admin_user" {
  username = var.portainer_username # optional, defaults is "admin"
  password = var.portainer_password
}
