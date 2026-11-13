data "portainer_kubesolo_versions" "this" {}

output "kubesolo_bundled" {
  value = data.portainer_kubesolo_versions.this.bundled
}

output "kubesolo_versions" {
  value = data.portainer_kubesolo_versions.this.versions
}
