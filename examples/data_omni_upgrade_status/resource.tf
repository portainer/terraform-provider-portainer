data "portainer_omni_upgrade_status" "talos" {
  credential_id = 1
  cluster       = "edge-fleet"
  component     = "talos"
}

output "omni_talos_upgrade_step" {
  value = data.portainer_omni_upgrade_status.talos.step
}

output "omni_talos_upgrade_available" {
  value = data.portainer_omni_upgrade_status.talos.available_versions
}
