resource "portainer_registry" "gitlab" {
  name         = var.gitlab_name
  url          = var.gitlab_url
  type         = 4
  username     = var.gitlab_username
  password     = var.gitlab_password
  instance_url = var.gitlab_instance_url
}
