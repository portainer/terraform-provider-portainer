resource "portainer_kubernetes_scoped_kubeconfig" "e2e" {
  environment_id = var.k8s_environment_id
  namespaces     = ["default"]
  expiry_seconds = 86400
  description    = "e2e"
}

output "issued" {
  value = length(nonsensitive(portainer_kubernetes_scoped_kubeconfig.e2e.kubeconfig)) > 0
}
