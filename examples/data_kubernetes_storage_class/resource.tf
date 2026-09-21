data "portainer_kubernetes_storage_class" "fast" {
  endpoint_id = 1
  name        = "standard"
}

output "kubernetes_storage_class_provisioner" {
  value = data.portainer_kubernetes_storage_class.fast.provisioner_name
}
