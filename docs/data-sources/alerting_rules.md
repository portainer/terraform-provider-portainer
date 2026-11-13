# Data Source Documentation: `portainer_alerting_rules`

# portainer_alerting_rules

> **Business Edition only.** This endpoint does not exist in Portainer CE.

Lists the alert rules - Portainer's predefined ones and custom ones - from `GET /observability/alerting/rules`, optionally filtered by name and category.

Rule IDs are assigned per Portainer instance, so this is the way to find a predefined rule for `portainer_alerting_rule` to adopt without hard-coding its `rule_id`.

## Example Usage

```hcl
data "portainer_alerting_rules" "node_not_ready" {
  name = "Node NotReady"
}

resource "portainer_alerting_rule" "node_not_ready" {
  rule_id = data.portainer_alerting_rules.node_not_ready.rules[0].rule_id
  enabled = true
}

data "portainer_alerting_rules" "security" {
  category = "security"
}

output "security_rule_names" {
  value = data.portainer_alerting_rules.security.rules[*].name
}
```

## Arguments Reference

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `name` | string | no | Only list the rules with exactly this name. |
| `category` | string | no | Only list the rules of this category: `portainer`, `security`, `environment`, `backup` or `addons`. |

## Attributes Reference

| Name | Type | Description |
|------|------|-------------|
| `rules` | list(object) | The matching rules, ordered by ID. |

### `rules`

Each element carries every attribute of the [`portainer_alerting_rule`](../resources/alerting_rule.md) resource except `created`:

| Name | Type | Description |
|------|------|-------------|
| `rule_id` | number | ID of the rule; pass it to `portainer_alerting_rule` to adopt the rule. |
| `name` | string | Name of the rule. |
| `description` | string | Description of the rule. |
| `summary` | string | Summary of the rule. |
| `category` | string | Category of the rule. |
| `enabled` | bool | Whether the rule is enabled. |
| `severity` | string | Severity of the rule. |
| `metric_type` | string | `percentage`, `bytes`, `raw` or `days`. |
| `condition_operator` | string | Comparison the threshold is evaluated with. |
| `operator_fixed` | bool | Whether the rule's definition fixes its condition operator. |
| `threshold` | number | Threshold of the rule. |
| `duration` | number | Period within which the condition must hold, in the rule's own unit. |
| `for_duration_minutes` | number | Minutes the condition has to keep holding before the alert fires. |
| `use_duration_as_for` | bool | Whether `duration` is used as the Prometheus `for` clause. |
| `alert_manager_id` | number | ID of the associated AlertManager settings. |
| `endpoint_group_ids` | list(number) | Endpoint groups the rule is restricted to. |
| `tag_ids` | list(number) | Environment tags the rule is restricted to. |
| `promql_expr` | string | PromQL expression of the rule. |
| `promql_template` | string | PromQL template of a built-in rule. |
| `promql_scalar_template` | string | Scalar PromQL template of a built-in rule. |
| `annotation_template` | string | Annotation template of the rule's alerts. |
| `use_tiers` | bool | Whether the rule is evaluated per severity tier. |
| `independent_tiers` | bool | Whether each tier measures its own series (BE 3.0+). |
| `tiers` | list(object) | Severity tiers: `severity`, `threshold`, `enabled`, `selector`. |
| `is_editable` | bool | Whether the rule can be edited. |
| `is_internal` | bool | Whether the rule is one of Portainer's built-in rules. |
| `labels` | map(string) | Labels of the rule. |
| `supported_agent_version` | string | Minimum agent version that supports the rule. |
| `supported_environment_types` | string | `docker`, `kubernetes`, `podman` or `all`. |
| `created_at` | string | When the rule was created. |
| `created_by` | string | Who created the rule. |
| `updated_at` | string | When the rule was last updated. |
