data "portainer_edge_stack" "example" {
  name = var.edge_stack_name
}

output "edge_stack_id" {
  value = data.portainer_edge_stack.example.id
}

output "edge_stack_deployment_type" {
  value = data.portainer_edge_stack.example.deployment_type
}
