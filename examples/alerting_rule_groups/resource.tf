resource "portainer_alerting_rule_groups" "cpu_saturation" {
  rule_id            = 7
  endpoint_group_ids = [1, 2]
}
