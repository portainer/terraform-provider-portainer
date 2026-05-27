data "portainer_stack" "example" {
  name        = var.stack_name
  endpoint_id = var.endpoint_id
}

output "stack_id" {
  value = data.portainer_stack.example.id
}

output "stack_type" {
  value = data.portainer_stack.example.type
}

output "stack_swarm_id" {
  value = data.portainer_stack.example.swarm_id
}
