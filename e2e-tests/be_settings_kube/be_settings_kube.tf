# The Kubernetes switches only Business Edition accepts.
resource "portainer_settings" "be" {
  disable_kube_roles_sync     = var.disabled
  disable_kube_shell          = var.disabled
  disable_kubeconfig_download = var.disabled
}
