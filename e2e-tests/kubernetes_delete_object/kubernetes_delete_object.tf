resource "portainer_kubernetes_delete_object" "remove_services" {
  environment_id = var.environment_id
  resource_type  = var.resource_type
  namespace      = var.namespace
  names          = var.names
}
