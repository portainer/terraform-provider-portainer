data "portainer_edge_group" "example" {
  name = var.edge_group_name
}

output "edge_group_id" {
  value = data.portainer_edge_group.example.id
}

output "edge_group_dynamic" {
  value = data.portainer_edge_group.example.dynamic
}
