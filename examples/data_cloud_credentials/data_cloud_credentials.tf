data "portainer_cloud_credentials" "example" {
  name = var.cloud_credentials_name
}

output "cloud_credentials_id" {
  value = data.portainer_cloud_credentials.example.id
}

output "cloud_credentials_provider" {
  value = data.portainer_cloud_credentials.example.cloud_provider
}
