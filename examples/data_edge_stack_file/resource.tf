data "portainer_edge_stack_file" "fleet" {
  edge_stack_id = 2
}

output "fleet_stack_body" {
  value = data.portainer_edge_stack_file.fleet.file_content
}
