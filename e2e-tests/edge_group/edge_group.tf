resource "portainer_edge_group" "example_dynamic" {
  name          = var.edge_group_name
  dynamic       = var.edge_group_dynamic
  partial_match = var.edge_group_partial_match
  tag_ids       = var.edge_group_tag_ids
}
