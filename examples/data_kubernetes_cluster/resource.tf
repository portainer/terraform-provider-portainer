data "portainer_kubernetes_cluster" "prod" {
  environment_id = var.environment_id
}

output "cluster_version" {
  value = data.portainer_kubernetes_cluster.prod.version
}

output "rbac_enabled" {
  value = data.portainer_kubernetes_cluster.prod.rbac_enabled
}

output "headroom" {
  value = "${data.portainer_kubernetes_cluster.prod.max_cpu}m CPU, ${data.portainer_kubernetes_cluster.prod.max_memory} bytes"
}
