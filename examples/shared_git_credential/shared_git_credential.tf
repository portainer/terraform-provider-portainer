resource "portainer_shared_git_credential" "test" {
  name               = var.git_credential_name
  username           = var.git_credential_username
  password           = var.git_credential_password
  authorization_type = var.git_credential_authorization_type
  url_pattern        = var.git_credential_url_pattern
}
