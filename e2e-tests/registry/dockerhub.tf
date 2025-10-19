resource "portainer_registry" "dockerhub" {
  name     = var.dockerhub_name
  type     = 6
  url      = var.dockerhub_url
  username = var.dockerhub_username
  password = var.dockerhub_password
}
