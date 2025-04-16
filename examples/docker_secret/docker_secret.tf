resource "portainer_docker_secret" "example_secret" {
  endpoint_id = var.endpoint_id
  name        = var.secret_name
  data        = var.secret_data

  labels     = var.secret_labels
  templating = var.secret_templating
}
