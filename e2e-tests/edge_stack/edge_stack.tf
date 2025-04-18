resource "portainer_edge_stack" "string_example" {
  name                    = var.edge_stack_name
  stack_file_content      = var.edge_stack_file_content
  deployment_type         = var.edge_stack_deployment_type
  edge_groups             = var.edge_stack_edge_groups
  registries              = var.edge_stack_registries
  use_manifest_namespaces = var.edge_stack_use_manifest_namespaces
}
