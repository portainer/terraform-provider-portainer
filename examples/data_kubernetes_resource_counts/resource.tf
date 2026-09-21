data "portainer_kubernetes_resource_counts" "prod" {
  endpoint_id = 1
}

output "kubernetes_cluster_size" {
  value = data.portainer_kubernetes_resource_counts.prod
}
