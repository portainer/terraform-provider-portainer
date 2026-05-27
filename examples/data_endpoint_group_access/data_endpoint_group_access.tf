data "portainer_endpoint_group_access" "example" {
  endpoint_group_id = var.endpoint_group_access_endpoint_group_id
  team_id           = var.endpoint_group_access_team_id
}

output "endpoint_group_access_id" {
  value = data.portainer_endpoint_group_access.example.id
}

output "endpoint_group_access_role_id" {
  value = data.portainer_endpoint_group_access.example.role_id
}
