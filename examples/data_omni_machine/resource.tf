data "portainer_omni_machine" "cp_1" {
  credential_id = 1
  machine_name  = "cp-1"
}

output "omni_machine_system_disk" {
  value = one([for d in data.portainer_omni_machine.cp_1.block_devices : d.linux_name if d.system_disk])
}

output "omni_machine_links" {
  value = [for l in data.portainer_omni_machine.cp_1.network_links : l.linux_name if l.link_up]
}
