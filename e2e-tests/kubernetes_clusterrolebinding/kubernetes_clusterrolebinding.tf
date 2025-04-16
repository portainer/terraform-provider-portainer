resource "portainer_kubernetes_clusterrolebinding" "example" {
  endpoint_id = var.endpoint_id
  manifest    = file(var.manifest_file)
}
