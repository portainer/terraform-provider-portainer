# Resource Documentation: `portainer_alerting_rule_groups`

# portainer_alerting_rule_groups

> **Business Edition only.** This endpoint does not exist in Portainer CE.


Attaches an alert rule to endpoint groups, which is what decides where the rule is evaluated.

This is a resource of its own rather than an argument on `portainer_alerting_rule` because most rules worth attaching are Portainer's own built-in ones. Terraform did not create them and cannot, but which environments they cover is exactly what belongs under version control.

## Example Usage

```hcl
resource "portainer_alerting_rule_groups" "cpu_saturation" {
  rule_id            = 7
  endpoint_group_ids = [portainer_endpoint_group.shops.id, portainer_endpoint_group.warehouses.id]
}
```

Attaching a rule Terraform created:

```hcl
resource "portainer_alerting_rule_groups" "custom" {
  rule_id            = portainer_alerting_rule.disk_pressure.rule_id
  endpoint_group_ids = [portainer_endpoint_group.shops.id]
}
```

## Arguments Reference

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `rule_id` | number | ✅ yes | Identifier of the alert rule. Changing it forces a new resource. |
| `endpoint_group_ids` | list(number) | ✅ yes | Endpoint groups the rule applies to. The list is authoritative: a group missing from it is detached. |

Portainer replaces the attachments on every write, so an empty list is a real request to detach the rule from everything rather than something the provider leaves out. That is also what destroying the resource sends - the attachments go away, the rule itself is left alone.

Attaching a rule is not the same as the rule being evaluated. Use the `portainer_alerting_rule_environments` data source to see where it actually landed.

## Import

```bash
terraform import portainer_alerting_rule_groups.cpu_saturation 7
```

The import ID is the alert rule identifier.
