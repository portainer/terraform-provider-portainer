# Data Source Documentation: `portainer_alerting_rule_environments`

# portainer_alerting_rule_environments

> **Business Edition only.** This endpoint does not exist in Portainer CE.


Reports where an alert rule actually landed, broken down by endpoint group.

Attaching a rule to an endpoint group is not the same as the rule being evaluated there: an agent can be too old, or an environment the wrong type. This data source is what tells you whether the attachment did anything.

## Example Usage

```hcl
data "portainer_alerting_rule_environments" "cpu_saturation" {
  rule_id = portainer_alerting_rule_groups.cpu_saturation.rule_id
}

output "alert_rule_undelivered" {
  value = data.portainer_alerting_rule_environments.cpu_saturation.error
}
```

## Arguments Reference

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `rule_id` | number | ✅ yes | Identifier of the alert rule to report on. |

## Attributes Reference

| Name | Type | Description |
|------|------|-------------|
| `total` | number | Environments the rule reaches, across every status below. |
| `active` | number | Environments where the rule is being evaluated. |
| `pending` | number | Environments where the rule has not been delivered yet. |
| `error` | number | Environments where delivering the rule failed. |
| `excluded` | number | Environments deliberately left out of the rule. |
| `not_supported` | number | Environments that cannot run the rule, typically an agent too old or an environment of the wrong type. |
| `groups` | list(object) | Delivery status broken down by endpoint group. |

### `groups`

| Name | Type | Description |
|------|------|-------------|
| `endpoint_group_id` | number | Identifier of the endpoint group. |
| `endpoint_group_name` | string | Name of the endpoint group. |
| `size` | number | Number of environments in the group. |
| `environments` | list(object) | The environments in the group and how the rule fared on each. |

### `groups.environments`

| Name | Type | Description |
|------|------|-------------|
| `endpoint_id` | number | Identifier of the environment. |
| `name` | string | Name of the environment. |
| `status` | string | Delivery status of the rule on this environment. |
| `reason_code` | string | Machine-readable reason behind the status. Branch on this rather than on `message`. |
| `message` | string | Human-readable explanation of the status. |
| `updated_at` | number | Unix timestamp of the last status change. |

A non-zero `error` is the number worth alerting on: the rule is attached but is not running where it was meant to.
