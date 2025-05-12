resource "portainer_open_amt_devices_action" "power_on" {
  environment_id = var.environment_id
  device_id      = var.device_id
  action         = var.device_action
}
