# Resource Documentation: `portainer_alerting_rule`

# portainer_alerting_rule

> **Business Edition only.** Alerting does not exist in Portainer CE.

The `portainer_alerting_rule` resource manages a Portainer alert rule. It works in two modes, chosen by `rule_id`:

- **Adopt** (`rule_id` set): takes over an existing rule - typically one of the rules Portainer predefines, which cannot be created - and applies the configured attributes to it.
- **Create** (`rule_id` unset): creates a custom rule with `POST /observability/alerting/rules`. `name` is required; the new rule's ID is reported in `rule_id`.

> **Changed in v3.0.0: destroy no longer deletes adopted rules.** Only a rule this resource created (`created = true`) is deleted from Portainer on destroy. An adopted or imported rule - including every rule managed with provider v2, whose state has no `created` - is only removed from the Terraform state and stays in Portainer as it is. Provider v2 sent `DELETE` for every rule. To switch a predefined rule off, set `enabled = false` instead of destroying the resource.

## Example Usage

### Create a custom rule
```hcl
resource "portainer_alerting_rule" "disk_almost_full" {
  name                        = "Disk almost full"
  description                 = "Root filesystem above 90% for 10 minutes."
  category                    = "environment"
  enabled                     = true
  severity                    = "warning"
  metric_type                 = "percentage"
  promql_expr                 = "100 - (node_filesystem_avail_bytes{mountpoint=\"/\"} / node_filesystem_size_bytes{mountpoint=\"/\"} * 100)"
  condition_operator          = ">"
  threshold                   = 90
  for_duration_minutes        = 10
  supported_environment_types = "kubernetes"
  endpoint_group_ids          = [2]

  labels = {
    team = "platform"
  }
}
```

### Adopt a predefined rule
```hcl
data "portainer_alerting_rules" "node_not_ready" {
  name = "Node NotReady"
}

resource "portainer_alerting_rule" "node_not_ready" {
  rule_id  = data.portainer_alerting_rules.node_not_ready.rules[0].rule_id
  enabled  = true
  severity = "critical"
  duration = 5
}
```

### Disable a predefined rule
```hcl
resource "portainer_alerting_rule" "backup_failure" {
  rule_id = 2
  enabled = false
}
```

## Lifecycle & Behavior
- **Create**: with `rule_id`, adopts the rule and updates it as below. Without it, sends `POST /observability/alerting/rules` (body `{"AlertingRule": {...}}`) and takes the ID from the response; should the response not carry it, the newest non-built-in rule with the configured `name` is looked up in `GET /observability/alerting/rules`.
- **Read**: Reads the rule from `GET /observability/alerting/rules/{id}`.
- **Update**: Reads the rule with `GET /observability/alerting/rules/{id}`, lays the configured attributes over it and sends the whole rule back with `PUT /observability/alerting/rules/{id}`. Tier `selector`s are left out, since Portainer never accepts them from a client.
- **Delete**: Sends `DELETE /observability/alerting/rules/{id}` only when `created` is true; otherwise only removes the rule from state (see the note above).

Only `enabled` and the attributes your configuration sets are changed. A create sends nothing else; an update sends everything else back as Portainer returned it, so adopting a predefined rule never touches its PromQL, templates, tiers or any field you leave out. All optional attributes are also computed: removing one from the configuration keeps the current value rather than resetting it.

Endpoint group attachments and severity tiers have resources of their own, `portainer_alerting_rule_groups` and `portainer_alerting_rule_tiers`, which also work on predefined rules. Set `endpoint_group_ids` here or use `portainer_alerting_rule_groups` for a rule, not both.

## Arguments Reference

### Main Attributes
| Name                          | Type         | Required | Description |
|-------------------------------|--------------|----------|-------------|
| `rule_id`                     | number       | No       | ID of an existing rule to adopt. Leave unset to create a custom rule (forces new resource if changed). |
| `enabled`                     | bool         | Yes      | Whether the alert rule is enabled. |
| `name`                        | string       | No       | Name of the alert rule. Required when `rule_id` is unset. |
| `description`                 | string       | No       | Description of the alert rule. |
| `summary`                     | string       | No       | Summary of the alert rule. |
| `category`                    | string       | No       | `portainer`, `security`, `environment`, `backup` or `addons` (`backup` and `addons` need BE 3.0+). |
| `severity`                    | string       | No       | Severity level: `critical`, `warning`, or `info`. |
| `metric_type`                 | string       | No       | Metric type: `percentage`, `bytes`, `raw` or `days`. |
| `condition_operator`          | string       | No       | Condition operator: `>`, `<`, `=`, `>=`, `<=`. Rejected for rules with `operator_fixed`. |
| `threshold`                   | float        | No       | Threshold value for the alert condition. |
| `duration`                    | number       | No       | Period within which the condition and threshold must hold, in the unit the rule itself uses. |
| `for_duration_minutes`        | number       | No       | Minutes the condition has to keep holding before the alert fires (the Prometheus `for` clause). |
| `promql_expr`                 | string       | No       | PromQL expression the rule evaluates; what a custom rule measures. |
| `alert_manager_id`            | number       | No       | ID of the associated AlertManager settings. |
| `endpoint_group_ids`          | list(number) | No       | Endpoint groups the rule is restricted to; empty means every relevant environment. |
| `tag_ids`                     | list(number) | No       | Environment tags the rule is restricted to. |
| `supported_environment_types` | string       | No       | `docker`, `kubernetes`, `podman` or `all`. |
| `labels`                      | map(string)  | No       | Labels associated with the alert rule. |

## Attributes Reference (Computed)
| Name                      | Type         | Description |
|---------------------------|--------------|-------------|
| `id`                      | string       | The rule ID (same as `rule_id`). |
| `created`                 | bool         | Whether Terraform created the rule. Only then does destroy delete it from Portainer. `false` after adoption or import. |
| `is_editable`             | bool         | Whether the rule can be edited. |
| `is_internal`             | bool         | Whether the rule is one of Portainer's built-in rules. |
| `operator_fixed`          | bool         | Whether the rule's definition fixes its condition operator. |
| `use_duration_as_for`     | bool         | Whether the rule uses `duration` as its Prometheus `for` clause. |
| `use_tiers`               | bool         | Whether the rule is evaluated per severity tier. |
| `independent_tiers`       | bool         | Whether each tier measures its own series (BE 3.0+). |
| `tiers`                   | list(object) | Severity tiers: `severity`, `threshold`, `enabled`, `selector` (BE 3.0+). Configure them with `portainer_alerting_rule_tiers`. |
| `promql_template`         | string       | PromQL template of a built-in rule. |
| `promql_scalar_template`  | string       | Scalar PromQL template of a built-in rule. |
| `annotation_template`     | string       | Annotation template of the rule's alerts. |
| `supported_agent_version` | string       | Minimum agent version that supports this rule. |
| `created_at`              | string       | Timestamp when the rule was created. |
| `created_by`              | string       | User who created the rule. |
| `updated_at`              | string       | Timestamp when the rule was last updated. |

## Import
Alert rules can be imported using the rule ID:
```bash
terraform import portainer_alerting_rule.high_cpu <rule-id>
```

An imported rule has `created = false`: destroying the resource leaves the rule in Portainer.
