data "portainer_kubernetes_storage_classes" "all" {
  endpoint_id = 1
}

output "kubernetes_default_storage_class" {
  value = one([for c in data.portainer_kubernetes_storage_classes.all.storage_classes : c.name if c.is_default])
}
