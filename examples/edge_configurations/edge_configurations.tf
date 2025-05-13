resource "portainer_edge_configurations" "example" {
  name           = var.edge_config_name
  type           = var.edge_config_type
  category       = var.edge_config_category
  base_dir       = var.edge_config_base_dir
  edge_group_ids = var.edge_group_ids
  file_path      = var.edge_config_file_path
  state          = var.edge_config_state
}
