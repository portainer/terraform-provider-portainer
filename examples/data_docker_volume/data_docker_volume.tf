data "portainer_docker_volume" "example" {
  endpoint_id = var.endpoint_id
  name        = var.docker_volume_name
}

output "docker_volume_id" {
  value = data.portainer_docker_volume.example.id
}

output "docker_volume_driver" {
  value = data.portainer_docker_volume.example.driver
}

output "docker_volume_mount_point" {
  value = data.portainer_docker_volume.example.mount_point
}
