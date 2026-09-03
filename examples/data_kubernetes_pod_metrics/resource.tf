data "portainer_kubernetes_pod_metrics" "prod" {
  environment_id = var.environment_id
  namespace      = "prod"
}

output "pod_cpu" {
  value = {
    for p in data.portainer_kubernetes_pod_metrics.prod.pods :
    p.name => join(", ", [for c in p.containers : "${c.name}=${c.cpu}"])
  }
}
