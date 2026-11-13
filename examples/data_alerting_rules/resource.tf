data "portainer_alerting_rules" "by_name" {
  name = var.rule_name
}

data "portainer_alerting_rules" "security" {
  category = "security"
}

output "alerting_rule_ids" {
  value = data.portainer_alerting_rules.by_name.rules[*].rule_id
}

output "security_alerting_rule_names" {
  value = data.portainer_alerting_rules.security.rules[*].name
}
