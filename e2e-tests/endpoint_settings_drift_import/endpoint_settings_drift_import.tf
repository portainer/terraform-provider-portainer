# The values are the ones the e2e Portainer fixture ships with, so the test
# leaves the environment as it found it.
resource "portainer_endpoint_settings" "drift" {
  endpoint_id = var.endpoint_id

  security_settings {
    allow_bind_mounts            = true
    allow_container_capabilities = true
    allow_device_mapping         = true
    allow_host_namespace         = true
    allow_privileged_mode        = true
    allow_stack_management       = true
    allow_sysctl_setting         = true
    allow_security_opt           = false
    allow_volume_browser         = false
    enable_host_management       = false
  }
}
