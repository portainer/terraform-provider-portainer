data "portainer_docker_image" "example" {
  endpoint_id = var.endpoint_id
  name        = var.docker_image_name
}

output "docker_image_id" {
  value = data.portainer_docker_image.example.id
}

output "docker_image_name" {
  value = data.portainer_docker_image.example.name
}
