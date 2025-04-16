resource "portainer_kubernetes_storage" "example" {
  endpoint_id = var.endpoint_id
  manifest    = file(var.manifest_file)
}
