resource "portainer_helm_user_repository" "test" {
  user_id = var.user_id
  url     = var.helm_repository_url
}
