resource "portainer_docker_volume" "example" {
  endpoint_id = var.endpoint_id
  name        = var.volume_name
  driver      = var.volume_driver

  driver_opts = var.volume_driver_opts
  labels      = var.volume_labels
}
