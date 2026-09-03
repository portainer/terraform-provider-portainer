data "portainer_system" "this" {}

output "portainer_version" {
  value = "${data.portainer_system.this.server_version} (${data.portainer_system.this.server_edition}, ${data.portainer_system.this.version_support})"
}

output "portainer_scale" {
  value = "${data.portainer_system.this.agents} agents, ${data.portainer_system.this.edge_agents} edge agents, ${data.portainer_system.this.nodes} nodes"
}

output "update_available" {
  value = data.portainer_system.this.update_available
}
