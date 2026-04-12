# Data Source Documentation: `portainer_policy`

# portainer_policy
The `portainer_policy` data source allows you to look up an existing Portainer Fleet Governance Policy by ID or name.

## Example Usage

### Look up a policy by name

```hcl
data "portainer_policy" "k8s_rbac" {
  name = "Production RBAC"
}

output "policy_id" {
  value = data.portainer_policy.k8s_rbac.id
}
```

### Look up a policy by ID

```hcl
data "portainer_policy" "example" {
  policy_id = 42
}

output "policy_name" {
  value = data.portainer_policy.example.name
}
```

## Arguments Reference

Exactly one of `policy_id` or `name` must be specified.

| Name        | Type   | Required | Description                               |
|-------------|--------|----------|-------------------------------------------|
| `policy_id` | int    | No       | ID of the policy to look up.              |
| `name`      | string | No       | Name of the policy to look up.            |

## Attributes Reference

| Name                 | Type   | Description                                     |
|----------------------|--------|-------------------------------------------------|
| `id`                 | string | ID of the policy.                               |
| `name`               | string | Name of the policy.                             |
| `environment_type`   | string | Environment type of the policy.                 |
| `policy_type`        | string | Policy type.                                    |
| `environment_groups` | list   | List of environment group IDs.                  |
| `data`               | string | Policy data as a JSON string.                   |
| `created_at`         | string | Timestamp when the policy was created.          |
| `updated_at`         | string | Timestamp when the policy was last updated.     |
