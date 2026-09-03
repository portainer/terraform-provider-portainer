data "portainer_kubernetes_persistent_volumes" "prod" {
  environment_id = var.environment_id
}

# Storage whose claim is gone but which a Retain policy kept from being reclaimed.
output "orphaned_storage" {
  value = [
    for v in data.portainer_kubernetes_persistent_volumes.prod.persistent_volumes : v.name
    if v.phase == "Released"
  ]
}
