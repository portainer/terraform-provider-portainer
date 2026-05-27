data "portainer_registry_access" "example" {
  registry_id = var.registry_id
  endpoint_id = var.endpoint_id
  team_id     = var.team_id
}

output "registry_access_id" {
  value = data.portainer_registry_access.example.id
}

output "registry_access_role_id" {
  value = data.portainer_registry_access.example.role_id
}
