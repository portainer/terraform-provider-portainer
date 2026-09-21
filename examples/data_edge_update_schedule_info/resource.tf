data "portainer_edge_update_schedule_info" "fleet" {}

output "edge_agents_behind" {
  value = data.portainer_edge_update_schedule_info.fleet.outdated_count
}

output "edge_agent_versions" {
  value = data.portainer_edge_update_schedule_info.fleet.agent_versions
}
