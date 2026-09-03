# Roll a deployment back to the previous revision (revision = 0).
resource "portainer_kubernetes_deployment_rollback" "web_previous" {
  environment_id  = var.environment_id
  namespace       = "default"
  deployment_name = "web"
  revision        = 0
}

# Roll back to a specific revision discovered from the replica sets.
data "portainer_kubernetes_replicasets" "web" {
  environment_id = var.environment_id
  namespace      = "default"
  deployment     = "web"
}

output "web_available_revisions" {
  value = [for rs in data.portainer_kubernetes_replicasets.web.replicasets : rs.revision]
}
