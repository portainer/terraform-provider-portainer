resource "portainer_container_exec" "standalone" {
  endpoint_id  = var.portainer_exec_endpoint_id
  service_name = var.portainer_exec_service_name
  command      = var.portainer_exec_command
  user         = var.portainer_exec_user
}
