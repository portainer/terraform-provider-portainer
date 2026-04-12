resource "portainer_user_git_credential" "test" {
  user_id            = var.user_id
  name               = var.git_credential_name
  username           = var.git_credential_username
  password           = var.git_credential_password
  authorization_type = var.git_credential_authorization_type
}
