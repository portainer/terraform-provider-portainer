resource "portainer_kubernetes_namespace" "test" {
  environment_id = var.environment_id
  name           = var.namespace_name
  owner          = var.namespace_owner

  annotations = var.namespace_annotations

  resource_quota = var.namespace_resource_quota
}
