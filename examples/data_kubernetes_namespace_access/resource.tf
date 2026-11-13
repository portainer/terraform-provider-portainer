# Requires Portainer Business Edition 3.0 or later.
data "portainer_kubernetes_namespace_access" "default" {
  endpoint_id  = var.environment_id
  namespace_id = "default"
}

output "namespace_user_ids" {
  value = [for u in data.portainer_kubernetes_namespace_access.default.user_access : u.user_id]
}
