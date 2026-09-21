data "portainer_edge_update_previous_versions" "shops" {
  edge_group_ids = [1]
}

output "edge_previous_agent_versions" {
  value = data.portainer_edge_update_previous_versions.shops.previous_versions
}
