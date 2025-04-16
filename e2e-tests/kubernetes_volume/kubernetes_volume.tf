resource "portainer_kubernetes_volume" "example" {
  endpoint_id = var.endpoint_id
  namespace   = var.namespace
  type        = var.type
  manifest    = file(var.manifest_file)
}
