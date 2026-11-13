# Requires Portainer Business Edition 3.0 or later.
data "portainer_kubernetes_gateway_classes" "all" {
  environment_id = var.environment_id
}

output "gateway_class_names" {
  value = [for c in data.portainer_kubernetes_gateway_classes.all.gateway_classes : c.name]
}
