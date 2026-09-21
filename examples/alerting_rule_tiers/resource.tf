resource "portainer_alerting_rule_tiers" "cpu_saturation" {
  rule_id            = 7
  enabled            = true
  condition_operator = ">"
  duration           = 5

  tier {
    severity  = "critical"
    threshold = 90
  }

  tier {
    severity  = "warning"
    threshold = 75
  }
}

output "alerting_rule_uses_tiers" {
  value = portainer_alerting_rule_tiers.cpu_saturation.use_tiers
}
