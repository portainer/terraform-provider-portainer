# Requires Portainer Business Edition 3.0 or later.
data "portainer_kubernetes_scoped_kubeconfigs" "all" {}

output "active_scoped_kubeconfigs" {
  value = [for k in data.portainer_kubernetes_scoped_kubeconfigs.all.scoped_kubeconfigs : k.id if k.active]
}
