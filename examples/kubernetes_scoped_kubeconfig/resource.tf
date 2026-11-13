# Requires Portainer Business Edition 2.45.2 or later.
resource "portainer_kubernetes_scoped_kubeconfig" "ci" {
  environment_id = var.environment_id
  namespaces     = ["default"]
  expiry_seconds = 86400
  description    = "CI read access"
}

# The credential is returned only once, when it is issued.
output "scoped_kubeconfig" {
  value     = portainer_kubernetes_scoped_kubeconfig.ci.kubeconfig
  sensitive = true
}
