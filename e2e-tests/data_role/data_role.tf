data "portainer_role" "admin" {
  name = var.role_name
}

output "role_id" {
  value = data.portainer_role.admin.id
}

output "roles" {
  value = data.portainer_role.admin.roles
}
