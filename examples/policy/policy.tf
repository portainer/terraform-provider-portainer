resource "portainer_policy" "test" {
  name               = var.policy_name
  environment_type   = var.policy_environment_type
  policy_type        = var.policy_type
  environment_groups = var.policy_environment_groups
  data               = var.policy_data
}
