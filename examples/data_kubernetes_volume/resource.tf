data "portainer_kubernetes_volume" "data" {
  endpoint_id = 1
  namespace   = "default"
  volume      = "data"
}

output "kubernetes_volume_reclaim_policy" {
  value = data.portainer_kubernetes_volume.data.persistent_volume_reclaim_policy
}
