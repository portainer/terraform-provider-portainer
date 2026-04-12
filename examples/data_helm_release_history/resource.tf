data "portainer_helm_release_history" "history" {
  endpoint_id  = var.endpoint_id
  release_name = var.release_name
  namespace    = "default"
}

output "release_revisions" {
  value = data.portainer_helm_release_history.history.revisions
}
