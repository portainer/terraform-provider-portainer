resource "portainer_kubernetes_cronjob" "example" {
  endpoint_id = var.endpoint_id
  namespace   = var.namespace
  manifest    = file(var.manifest_file)
}
