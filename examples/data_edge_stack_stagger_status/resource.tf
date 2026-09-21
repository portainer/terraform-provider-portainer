data "portainer_edge_stack_stagger_status" "monitoring" {
  edge_stack_id = 1
}

output "edge_stack_stagger_status" {
  value = data.portainer_edge_stack_stagger_status.monitoring.status
}
