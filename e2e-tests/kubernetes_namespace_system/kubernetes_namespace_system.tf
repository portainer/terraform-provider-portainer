# Portainer never marks "default" as a system namespace, so the test uses a
# namespace of its own.
resource "portainer_kubernetes_namespace" "example" {
  environment_id = var.environment_id
  name           = var.namespace
}

resource "portainer_kubernetes_namespace_system" "example" {
  environment_id = var.environment_id
  namespace      = portainer_kubernetes_namespace.example.name
  system         = var.system
}
