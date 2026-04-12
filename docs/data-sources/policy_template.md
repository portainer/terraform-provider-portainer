# Data Source Documentation: `portainer_policy_template`

# portainer_policy_template
The `portainer_policy_template` data source allows you to look up a built-in Portainer Fleet Governance Policy Template by ID or name. Templates provide pre-configured policy definitions that can be used when creating policies.

## Example Usage

### Look up a policy template by name

```hcl
data "portainer_policy_template" "security_baseline" {
  name = "Security Baseline"
}

output "template_data" {
  value = data.portainer_policy_template.security_baseline.data
}
```

### Look up a policy template by ID

```hcl
data "portainer_policy_template" "example" {
  template_id = "security-k8s-baseline"
}

output "template_name" {
  value = data.portainer_policy_template.example.name
}
```

## Arguments Reference

Exactly one of `template_id` or `name` must be specified.

| Name          | Type   | Required | Description                                      |
|---------------|--------|----------|--------------------------------------------------|
| `template_id` | string | No       | ID of the policy template to look up.            |
| `name`        | string | No       | Name of the policy template to look up.          |

## Attributes Reference

| Name          | Type   | Description                                              |
|---------------|--------|----------------------------------------------------------|
| `id`          | string | ID of the policy template.                               |
| `name`        | string | Name of the policy template.                             |
| `description` | string | Description of the policy template.                      |
| `category`    | string | Category (rbac, security, setup, registry).              |
| `policy_type` | string | Policy type of the template.                             |
| `data`        | string | Template data as a JSON string.                          |
