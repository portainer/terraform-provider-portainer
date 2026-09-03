# Manage the replica count of a deployment that already exists in the cluster.
resource "portainer_kubernetes_deployment_scale" "web" {
  environment_id  = var.environment_id
  namespace       = "default"
  deployment_name = "web"
  replicas        = 3
}

output "web_ready_replicas" {
  value = portainer_kubernetes_deployment_scale.web.ready_replicas
}
