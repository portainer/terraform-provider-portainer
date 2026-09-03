data "portainer_kubernetes_resource_quotas" "default" {
  environment_id = var.environment_id
  namespace      = "default"
}

output "quota_limits" {
  value = {
    for q in data.portainer_kubernetes_resource_quotas.default.resource_quotas : q.name => q.hard
  }
}

output "quota_usage" {
  value = {
    for q in data.portainer_kubernetes_resource_quotas.default.resource_quotas : q.name => q.used
  }
}
