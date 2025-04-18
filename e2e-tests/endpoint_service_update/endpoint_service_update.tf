resource "portainer_endpoint_service_update" "force_update" {
  endpoint_id  = var.endpoint_id
  service_name = var.service_name
  pull_image   = var.pull_image
}
