data "portainer_omni_talos_versions" "supported" {
  credential_id = 1
}

output "omni_talos_versions" {
  value = data.portainer_omni_talos_versions.supported.talos_versions
}

output "omni_talos_compatibility" {
  value = data.portainer_omni_talos_versions.supported.compatibility
}
