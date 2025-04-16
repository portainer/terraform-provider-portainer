resource "portainer_auth" "login" {
  username = var.portainer_username
  password = var.portainer_password
}
