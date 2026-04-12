data "portainer_kubernetes_crd" "all" {
  environment_id = var.endpoint_id
}

output "crd_count" {
  value = length(data.portainer_kubernetes_crd.all.crds)
}

output "crds" {
  value = data.portainer_kubernetes_crd.all.crds
}
