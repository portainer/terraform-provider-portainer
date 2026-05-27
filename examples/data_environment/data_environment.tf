data "portainer_environment" "example" {
  name = var.environment_name
}

output "environment_id" {
  value = data.portainer_environment.example.id
}

output "environment_type" {
  value = data.portainer_environment.example.type
}

output "environment_group_id" {
  value = data.portainer_environment.example.group_id
}
