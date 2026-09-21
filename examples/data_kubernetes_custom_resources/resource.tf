data "portainer_kubernetes_custom_resources" "widgets" {
  endpoint_id = 1
  definition  = "widgets.example.com"
}

output "kubernetes_custom_resources" {
  value = data.portainer_kubernetes_custom_resources.widgets.resources
}
