data "portainer_policy" "example" {
  name = var.policy_name
}

output "policy_id" {
  value = data.portainer_policy.example.id
}

output "policy_environment_type" {
  value = data.portainer_policy.example.environment_type
}

output "policy_type" {
  value = data.portainer_policy.example.policy_type
}
