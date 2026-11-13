# Read-only APIs new in 3.0.
data "portainer_system" "this" {}
data "portainer_addon_catalog" "catalog" {}
data "portainer_kubesolo_versions" "this" {}
data "portainer_kubesolo_offline_bundles" "this" {}
data "portainer_kubernetes_scoped_kubeconfigs" "all" {}

data "portainer_kubernetes_namespace_access" "default" {
  endpoint_id  = var.k8s_environment_id
  namespace_id = "default"
}

output "edition" {
  value = data.portainer_system.this.server_edition
}

output "addons_availability" {
  value = data.portainer_system.this.addons_availability
}

output "kubesolo_bundled" {
  value = data.portainer_kubesolo_versions.this.bundled
}
