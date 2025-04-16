resource "portainer_user" "your-user" {
  username = var.portainer_user_username
  password = var.portainer_user_password
  role     = var.portainer_user_role
}