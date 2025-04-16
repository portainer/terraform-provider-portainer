resource "portainer_endpoint_settings" "test" {
  endpoint_id                  = var.endpoint_id
  allow_bind_mounts            = var.allow_bind_mounts
  allow_container_capabilities = var.allow_container_capabilities
  allow_device_mapping         = var.allow_device_mapping
  allow_host_namespace         = var.allow_host_namespace
  allow_privileged_mode        = var.allow_privileged_mode
  allow_stack_management       = var.allow_stack_management
  allow_sysctl_setting         = var.allow_sysctl_setting
  allow_volume_browser         = var.allow_volume_browser
  enable_gpu_management        = var.enable_gpu_management
  enable_host_management       = var.enable_host_management

  dynamic "gpus" {
    for_each = var.gpus
    content {
      name  = gpus.value.name
      value = gpus.value.value
    }
  }
}
