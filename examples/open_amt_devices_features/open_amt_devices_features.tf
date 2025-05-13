resource "portainer_open_amt_devices_features" "example" {
  environment_id = var.environment_id
  device_id      = var.device_id

  ider         = var.ider
  kvm          = var.kvm
  sol          = var.sol
  redirection  = var.redirection
  user_consent = var.user_consent
}
