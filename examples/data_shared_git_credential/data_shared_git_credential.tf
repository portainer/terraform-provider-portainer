data "portainer_shared_git_credential" "example" {
  name = var.git_credential_name
}

output "credential_id" {
  value = data.portainer_shared_git_credential.example.id
}

output "credential_username" {
  value = data.portainer_shared_git_credential.example.username
}

output "credential_authorization_type" {
  value = data.portainer_shared_git_credential.example.authorization_type
}
