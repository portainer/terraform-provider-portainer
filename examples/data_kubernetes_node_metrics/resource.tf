data "portainer_kubernetes_node_metrics" "worker" {
  environment_id = var.environment_id
  node           = "worker-1"
}

output "worker_cpu" {
  value = data.portainer_kubernetes_node_metrics.worker.cpu
}
