data "portainer_kubernetes_config" "prod" {
  environment_ids = [var.environment_id]
}

# The kubeconfig carries a bearer token, so it is written out as a sensitive file.
output "kubeconfig" {
  value     = data.portainer_kubernetes_config.prod.kubeconfig
  sensitive = true
}
