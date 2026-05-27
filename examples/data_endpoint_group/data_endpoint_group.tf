data "portainer_endpoint_group" "example" {
  name = var.endpoint_group_name
}

output "endpoint_group_id" {
  value = data.portainer_endpoint_group.example.id
}

output "endpoint_group_description" {
  value = data.portainer_endpoint_group.example.description
}
