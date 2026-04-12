resource "portainer_alerting_rule" "test" {
  rule_id   = var.rule_id
  enabled   = var.rule_enabled
  threshold = var.rule_threshold
  duration  = var.rule_duration
}
