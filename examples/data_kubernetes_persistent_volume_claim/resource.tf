data "portainer_kubernetes_persistent_volume_claim" "data" {
  endpoint_id = 1
  namespace   = "default"
  name        = "data"
}

output "kubernetes_claim_phase" {
  value = data.portainer_kubernetes_persistent_volume_claim.data.phase
}
