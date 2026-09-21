data "portainer_kubernetes_volumes" "all" {
  endpoint_id       = 1
  with_applications = true
}

output "kubernetes_volumes_json" {
  value = data.portainer_kubernetes_volumes.all.volumes
}
