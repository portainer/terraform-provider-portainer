data "portainer_alerting_rule_environments" "cpu_saturation" {
  rule_id = 7
}

output "alerting_rule_active_environments" {
  value = data.portainer_alerting_rule_environments.cpu_saturation.active
}

output "alerting_rule_undelivered_environments" {
  value = data.portainer_alerting_rule_environments.cpu_saturation.error
}
