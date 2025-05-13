resource "portainer_edge_group" "example_static" {
  name          = var.edge_group_name
  dynamic       = var.edge_group_dynamic
  partial_match = var.edge_group_partial_match
  tag_ids       = var.edge_group_tag_ids
}

resource "portainer_edge_stack" "repository_example" {
  name                    = var.edge_stack_name
  repository_url          = var.edge_stack_repository_url
  file_path_in_repository = var.edge_stack_file_path_in_repository
  deployment_type         = var.edge_stack_deployment_type
  edge_groups             = [portainer_edge_group.example_static.id]
  registries              = var.edge_stack_registries
  use_manifest_namespaces = var.edge_stack_use_manifest_namespaces
}
