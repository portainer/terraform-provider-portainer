resource "portainer_kubernetes_rolebinding" "example" {
  endpoint_id = var.endpoint_id
  namespace   = var.namespace
  manifest    = file(var.manifest_file)
}
