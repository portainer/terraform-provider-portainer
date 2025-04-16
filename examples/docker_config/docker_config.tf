resource "portainer_docker_config" "example_config" {
  endpoint_id = var.endpoint_id
  name        = var.config_name
  data        = var.config_data

  labels     = var.config_labels
  templating = var.config_templating
}
