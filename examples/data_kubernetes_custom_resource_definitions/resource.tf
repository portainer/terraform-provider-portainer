data "portainer_kubernetes_custom_resource_definitions" "all" {
  endpoint_id = 1
}

output "kubernetes_crd_names" {
  value = [for c in data.portainer_kubernetes_custom_resource_definitions.all.definitions : c.name]
}
