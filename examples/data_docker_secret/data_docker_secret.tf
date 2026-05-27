data "portainer_docker_secret" "example" {
  endpoint_id = var.endpoint_id
  name        = var.docker_secret_name
}

output "docker_secret_id" {
  value = data.portainer_docker_secret.example.id
}

output "docker_secret_name" {
  value = data.portainer_docker_secret.example.name
}
