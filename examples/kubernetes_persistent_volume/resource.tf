# Adopts a PersistentVolume created elsewhere and pins its reclaim policy so the
# underlying storage survives the claim being deleted.
resource "portainer_kubernetes_persistent_volume" "data" {
  environment_id = 1
  name           = "pvc-8f3c1a2e-data"
  reclaim_policy = "Retain"
}

output "data_volume_claim" {
  value = portainer_kubernetes_persistent_volume.data.claim_ref
}
