resource "portainer_helm_rollback" "rollback" {
  endpoint_id  = var.endpoint_id
  release_name = var.release_name
  namespace    = "default"
  revision     = var.revision
}
