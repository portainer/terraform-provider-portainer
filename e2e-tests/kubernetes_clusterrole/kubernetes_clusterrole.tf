resource "portainer_kubernetes_clusterrole" "example" {
  endpoint_id = var.endpoint_id
  manifest    = file(var.manifest_file)
}
