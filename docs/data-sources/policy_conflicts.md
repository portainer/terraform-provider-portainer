# Data Source Documentation: `portainer_policy_conflicts`

# portainer_policy_conflicts

> **Business Edition only.** This endpoint does not exist in Portainer CE.


Previews what a policy would collide with, and how many environments could actually run it, before it is created.

## Example Usage

```hcl
data "portainer_policy_conflicts" "baseline" {
  policy_json = jsonencode({
    name                = "baseline-v2"
    environmentGroupIds = [portainer_endpoint_group.shops.id]
  })
}

output "policy_would_conflict_with" {
  value = [for c in data.portainer_policy_conflicts.baseline.conflicts : c.existing_policy_name]
}
```

## Arguments Reference

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `policy_json` | string | ✅ yes | The candidate policy, as a JSON object. Use `jsonencode({...})`. Portainer does not describe this payload's shape in its API specification, so it is passed through as given. |

## Attributes Reference

| Name | Type | Description |
|------|------|-------------|
| `conflicts` | list(object) | Environment groups where an existing policy already applies. |
| `new_groups` | list(object) | Environment groups the candidate policy would newly cover, with no existing policy in the way. |
| `supported_environments` | number | Of those, the ones that can run it. |
| `total_environments` | number | Environments the candidate policy would reach. |
| `unsupported_environments` | number | Of those, the ones that cannot - usually an agent too old for the policy type. Check `portainer_policy_metadata` for the versions each type needs. |

### `conflicts`

| Name | Type | Description |
|------|------|-------------|
| `environment_count` | number | Environments in that group. |
| `environment_group_id` | number | Identifier of the environment group. |
| `environment_group_name` | string | Name of the environment group. |
| `existing_policy_id` | number | Identifier of the policy already applying there. |
| `existing_policy_name` | string | Name of that policy. |
| `supported_environments` | number | Of those, the ones that can run the policy. |
| `unsupported_environments` | number | Of those, the ones that cannot. |

### `new_groups`

| Name | Type | Description |
|------|------|-------------|
| `environment_count` | number | Environments in that group. |
| `environment_group_id` | number | Identifier of the environment group. |
| `environment_group_name` | string | Name of the environment group. |
| `supported_environments` | number | Of those, the ones that can run the policy. |
| `unsupported_environments` | number | Of those, the ones that cannot. |

Portainer does not describe this request's shape in its API specification, so `policy_json` is passed through exactly as given rather than flattened into arguments that could not be kept accurate.

This is a data source rather than a resource because the endpoint changes nothing - it answers a question about a candidate policy.
