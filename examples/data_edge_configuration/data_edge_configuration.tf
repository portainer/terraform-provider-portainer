data "portainer_edge_configuration" "example" {
  name = var.edge_configuration_name
}

output "edge_configuration_type" {
  description = "Numeric type of the looked-up edge configuration"
  value       = data.portainer_edge_configuration.example.type
}

output "edge_configuration_category" {
  description = "Category of the looked-up edge configuration"
  value       = data.portainer_edge_configuration.example.category
}
