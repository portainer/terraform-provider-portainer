data "portainer_omni_machines" "all" {
  credential_id = 1
}

output "omni_unallocated_machines" {
  value = [for m in data.portainer_omni_machines.all.machines : m.machine_name if m.cluster == ""]
}
