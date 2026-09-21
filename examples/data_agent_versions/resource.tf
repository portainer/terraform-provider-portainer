data "portainer_agent_versions" "fleet" {}

output "agent_versions" {
  value = data.portainer_agent_versions.fleet.versions
}
