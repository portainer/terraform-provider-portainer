data "portainer_kubernetes_custom_resource_definition" "widgets" {
  endpoint_id = 1
  name        = "widgets.example.com"
}

output "kubernetes_crd_scope" {
  value = data.portainer_kubernetes_custom_resource_definition.widgets.scope
}
