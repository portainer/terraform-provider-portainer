resource "portainer_kubernetes_namespace_system" "example" {
  environment_id = var.environment_id
  namespace      = var.namespace
  system         = var.system
}
