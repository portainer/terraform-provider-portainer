data "portainer_kubernetes_gpu" "prod" {
  endpoint_id = 1
}

output "kubernetes_gpu_nodes" {
  value = data.portainer_kubernetes_gpu.prod.gpu_node_count
}

output "kubernetes_gpu_workloads_stuck" {
  value = [
    for w in data.portainer_kubernetes_gpu.prod.workloads : w.pod_name
    if w.scheduling_issue != ""
  ]
}
