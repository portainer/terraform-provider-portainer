# Cluster-wide: omit namespace to list pods in every accessible namespace.
data "portainer_kubernetes_pods" "not_running" {
  environment_id = var.environment_id
  field_selector = "status.phase!=Running"
}

data "portainer_kubernetes_pods" "web" {
  environment_id = var.environment_id
  namespace      = "default"
  label_selector = "app=web"
}

output "unhealthy_pods" {
  value = [
    for p in data.portainer_kubernetes_pods.not_running.pods : "${p.namespace}/${p.name} (${p.phase})"
  ]
}

output "web_pods_ready" {
  value = alltrue([for p in data.portainer_kubernetes_pods.web.pods : p.ready])
}
