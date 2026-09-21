data "portainer_kubernetes_persistent_volume_claims" "all" {
  endpoint_id = 1
}

output "kubernetes_unbound_claims" {
  value = [
    for c in data.portainer_kubernetes_persistent_volume_claims.all.claims : "${c.namespace}/${c.name}"
    if c.phase != "Bound"
  ]
}
