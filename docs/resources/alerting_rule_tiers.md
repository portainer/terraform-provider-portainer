# Resource Documentation: `portainer_alerting_rule_tiers`

# portainer_alerting_rule_tiers

> **Business Edition only.** This endpoint does not exist in Portainer CE.


Configures the multi-severity tiers of an editable built-in alert rule: the same metric firing at warning above one threshold and critical above another.

## Example Usage

```hcl
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
```

## Arguments Reference

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `rule_id` | number | ✅ yes | Identifier of the alert rule. Changing it forces a new resource. |
| `enabled` | bool | ❌ no | Whether the rule is enabled. Defaults to `true`. |
| `description` | string | ❌ no | Description stored on the rule. Leave unset to keep what the rule already has. |
| `condition_operator` | string | ❌ no | Comparison the thresholds are evaluated with: `>`, `<`, `=`, `>=` or `<=`. |
| `duration` | number | ❌ no | How long the condition has to hold before the rule fires. |
| `tier` | block | ✅ yes | Severity tiers of the rule, in order. Repeatable. |

### `tier`

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `severity` | string | ✅ yes | Severity this tier raises: `critical`, `warning` or `info`. |
| `threshold` | number | ✅ yes | Value the metric is compared against for this tier. |
| `enabled` | bool | ❌ no | Whether this tier is evaluated. Defaults to `true`. |

## Attributes Reference

| Name | Type | Description |
|------|------|-------------|
| `use_tiers` | bool | Whether Portainer actually evaluates the rule per tier. A rule reporting `false` is still using its single threshold. |

The tiers are a list rather than a set: they are an ordered ladder of severities, and reordering them is a change worth seeing in a plan.

`description`, `condition_operator` and `duration` are left out of the request while they are unset, because Portainer keeps the rule's own value for an omitted field - blanking a built-in rule's description is not what an apply about thresholds should do. Once read, they are tracked, so a later change to one of them is applied normally.

Portainer rejects this call for rules that are not editable internal rules, and some built-in rules fix their operator and reject a change to `condition_operator`.

Portainer has no endpoint to clear a tier configuration, so destroying the resource stops managing it and leaves the rule as configured.

## Import

```bash
terraform import portainer_alerting_rule_tiers.cpu_saturation 7
```

The import ID is the alert rule identifier.
