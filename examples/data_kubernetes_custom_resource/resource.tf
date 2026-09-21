data "portainer_kubernetes_custom_resource" "prod_widget" {
  endpoint_id = 1
  definition  = "widgets.example.com"
  namespace   = "apps"
  name        = "prod"
}

output "kubernetes_custom_resource_kind" {
  value = jsondecode(data.portainer_kubernetes_custom_resource.prod_widget.manifest).kind
}
