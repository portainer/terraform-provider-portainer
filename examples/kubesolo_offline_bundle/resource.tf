data "portainer_kubesolo_versions" "this" {}

# The tarball must belong to the KubeSolo release bundled with Portainer and
# keep the name it is published under.
resource "portainer_kubesolo_offline_bundle" "amd64" {
  name      = "kubesolo-${data.portainer_kubesolo_versions.this.bundled}-linux-amd64-offline.tar.gz"
  file_path = "${path.module}/kubesolo-${data.portainer_kubesolo_versions.this.bundled}-linux-amd64-offline.tar.gz"
}

output "bundle_size" {
  value = portainer_kubesolo_offline_bundle.amd64.size
}
