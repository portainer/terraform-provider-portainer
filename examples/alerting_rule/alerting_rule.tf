# Adopt an existing (predefined) rule: destroy leaves it in Portainer.
resource "portainer_alerting_rule" "test" {
  rule_id   = var.rule_id
  enabled   = var.rule_enabled
  threshold = var.rule_threshold
  duration  = var.rule_duration
}

# Create a custom rule: destroy deletes it from Portainer.
resource "portainer_alerting_rule" "custom" {
  name                 = var.custom_rule_name
  enabled              = true
  category             = "environment"
  severity             = "warning"
  metric_type          = "percentage"
  promql_expr          = var.custom_rule_promql_expr
  condition_operator   = ">"
  threshold            = 90
  for_duration_minutes = 10
}
