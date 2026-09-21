data "portainer_policy_metadata" "requirements" {}

output "policy_minimum_agents" {
  value = data.portainer_policy_metadata.requirements.minimum_agent_versions
}
