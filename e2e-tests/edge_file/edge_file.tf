resource "portainer_edge_group" "example_static" {
  name          = var.edge_group_name
  dynamic       = var.edge_group_dynamic
  partial_match = var.edge_group_partial_match
  tag_ids       = var.edge_group_tag_ids
}

resource "portainer_edge_stack" "string_example" {
  name            = var.edge_stack_name
  stack_file_path = file("${path.module}/${var.edge_stack_file_path}")
  deployment_type = var.edge_stack_deployment_type
  edge_groups     = [portainer_edge_group.example_static.id]
  registries      = var.edge_stack_registries
}
