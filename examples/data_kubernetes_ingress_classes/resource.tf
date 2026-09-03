data "portainer_kubernetes_ingress_classes" "all" {
  environment_id = var.environment_id
}

output "ingress_class_names" {
  value = [for c in data.portainer_kubernetes_ingress_classes.all.ingress_classes : c.name]
}

output "default_ingress_class" {
  value = data.portainer_kubernetes_ingress_classes.all.default_ingress_class
}
