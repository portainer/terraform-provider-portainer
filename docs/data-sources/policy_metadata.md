# Data Source Documentation: `portainer_policy_metadata`

# portainer_policy_metadata

> **Business Edition only.** This endpoint does not exist in Portainer CE.


Reports the lowest agent version each policy type needs.

## Example Usage

```hcl
data "portainer_policy_metadata" "requirements" {}

output "policy_minimum_agents" {
  value = data.portainer_policy_metadata.requirements.minimum_agent_versions
}
```

## Arguments Reference

| Name | Type | Required | Description |
|------|------|----------|-------------|

## Attributes Reference

| Name | Type | Description |
|------|------|-------------|
| `minimum_agent_versions` | map(string) | The lowest agent version each policy type needs, keyed by policy type. An environment on an older agent will not run that policy. |

An environment on an older agent than a policy type requires will not run that policy. `portainer_policy_conflicts` reports how many environments that affects for a given policy.
