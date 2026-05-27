data "portainer_registry" "example" {
  name = var.registry_name
}

output "registry_id" {
  value = data.portainer_registry.example.id
}

output "registry_url" {
  value = data.portainer_registry.example.url
}

output "registry_type" {
  value = data.portainer_registry.example.type
}
