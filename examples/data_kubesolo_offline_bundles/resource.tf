data "portainer_kubesolo_offline_bundles" "all" {}

output "offline_architectures" {
  value = [for b in data.portainer_kubesolo_offline_bundles.all.bundles : "${b.arch}/${b.libc}"]
}
