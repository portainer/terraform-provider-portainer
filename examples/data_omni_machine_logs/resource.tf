data "portainer_omni_machine_logs" "cp_1" {
  credential_id = 1
  machine_name  = "cp-1"
}

output "omni_machine_logs" {
  value = data.portainer_omni_machine_logs.cp_1.logs
}
