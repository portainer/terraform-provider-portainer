# A predefined rule adopted by ID, and a custom rule this resource creates.
data "portainer_alerting_rules" "all" {}

locals {
  adoptable = [for r in data.portainer_alerting_rules.all.rules : r if r.is_editable]
}

resource "portainer_alerting_rule" "adopted" {
  rule_id   = local.adoptable[0].rule_id
  enabled   = true
  threshold = var.threshold
}

resource "portainer_alerting_rule" "custom" {
  name                 = "e2e-custom"
  enabled              = true
  category             = "environment"
  severity             = "warning"
  metric_type          = "percentage"
  promql_expr          = "100 - (avg(rate(node_cpu_seconds_total{mode=\"idle\"}[5m])) * 100)"
  condition_operator   = ">"
  threshold            = var.threshold
  for_duration_minutes = 10
}

output "adopted_id" {
  value = portainer_alerting_rule.adopted.rule_id
}

output "custom_id" {
  value = portainer_alerting_rule.custom.rule_id
}
