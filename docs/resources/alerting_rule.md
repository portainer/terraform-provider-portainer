# Resource Documentation: `portainer_alerting_rule`

# portainer_alerting_rule
The `portainer_alerting_rule` resource manages Portainer alerting rules. Alert rules in Portainer are predefined and cannot be created via the API -- they can only be read, updated, and deleted.

To use this resource, specify the `rule_id` of an existing predefined rule. On `terraform apply`, the resource adopts the rule and applies the desired configuration via the update API.

## Example Usage

### Enable and configure a predefined alert rule
```hcl
resource "portainer_alerting_rule" "high_cpu" {
  rule_id            = 1
  enabled            = true
  severity           = "critical"
  condition_operator = ">"
  threshold          = 90.0
  duration           = 300
  alert_manager_id   = 1

  labels = {
    team = "platform"
  }
}
```

### Disable an alert rule
```hcl
resource "portainer_alerting_rule" "disk_space" {
  rule_id = 2
  enabled = false
}
```

## Lifecycle & Behavior
- **Create**: There is no POST endpoint. The resource adopts an existing rule by `rule_id` and immediately calls update.
- **Read**: Reads the rule from `GET /observability/alerting/rules/{id}`.
- **Update**: Sends `PUT /observability/alerting/rules/{id}` with the updated configuration.
- **Delete**: Sends `DELETE /observability/alerting/rules/{id}`.

## Arguments Reference

### Main Attributes
| Name                 | Type        | Required | Description                                                              |
|----------------------|-------------|----------|--------------------------------------------------------------------------|
| `rule_id`            | number      | Yes      | ID of the predefined alert rule to manage (forces new resource if changed). |
| `enabled`            | bool        | Yes      | Whether the alert rule is enabled.                                       |
| `name`               | string      | No       | Name of the alert rule.                                                  |
| `description`        | string      | No       | Description of the alert rule.                                           |
| `summary`            | string      | No       | Summary of the alert rule.                                               |
| `severity`           | string      | No       | Severity level: `critical`, `warning`, or `info`.                        |
| `metric_type`        | string      | No       | Metric type: `percentage`, `bytes`, or `raw`.                            |
| `condition_operator` | string      | No       | Condition operator: `>`, `<`, `=`, `>=`, `<=`.                           |
| `threshold`          | float       | No       | Threshold value for the alert condition.                                 |
| `duration`           | number      | No       | Duration (in seconds) the condition must persist before firing.          |
| `alert_manager_id`   | number      | No       | ID of the associated AlertManager settings.                              |
| `labels`             | map(string) | No       | Labels associated with the alert rule.                                   |

## Attributes Reference (Computed)
| Name                            | Type   | Description                                                        |
|---------------------------------|--------|--------------------------------------------------------------------|
| `id`                            | string | The rule ID (same as `rule_id`).                                   |
| `is_editable`                   | bool   | Whether the rule can be edited.                                    |
| `is_internal`                   | bool   | Whether the rule is an internal/system rule.                       |
| `supported_agent_version`       | string | Minimum agent version that supports this rule.                     |
| `supported_environment_types`   | string | Environment types that support this rule.                          |
| `created_at`                    | string | Timestamp when the rule was created.                               |
| `created_by`                    | string | User who created the rule.                                         |
| `updated_at`                    | string | Timestamp when the rule was last updated.                          |

## Import
Alert rules can be imported using the rule ID:
```bash
terraform import portainer_alerting_rule.high_cpu <rule-id>
```
