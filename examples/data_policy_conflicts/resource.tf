data "portainer_policy_conflicts" "baseline" {
  policy_json = jsonencode({
    name                = "baseline-v2"
    environmentGroupIds = [1]
  })
}

output "policy_conflicting_names" {
  value = [for c in data.portainer_policy_conflicts.baseline.conflicts : c.existing_policy_name]
}
