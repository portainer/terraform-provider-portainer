data "portainer_user" "example" {
  username = var.user_username
}

output "user_id" {
  value = data.portainer_user.example.id
}

output "user_role" {
  value = data.portainer_user.example.role
}
