resource "portainer_docker_plugin" "rclone" {
  endpoint_id = var.endpoint_id
  remote      = var.remote
  name        = var.name

  dynamic "settings" {
    for_each = var.settings
    content {
      name  = settings.value.name
      value = settings.value.value
    }
  }
}
