# Data Source Documentation: `portainer_edge_update_previous_versions`

# portainer_edge_update_previous_versions

> **Business Edition only.** This endpoint does not exist in Portainer CE.


Reports the agent version each environment ran before its last update, which is what a rollback would return it to.

## Example Usage

```hcl
data "portainer_edge_update_previous_versions" "shops" {
  edge_group_ids = [portainer_edge_group.shops.id]
}
```

## Arguments Reference

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `edge_group_ids` | list(string) | ❌ no | Only report on the environments in these edge groups. |
| `environment_ids` | list(string) | ❌ no | Only report on these environments. |

## Attributes Reference

| Name | Type | Description |
|------|------|-------------|
| `previous_versions` | list(object) | The agent version each environment was on before its last update, which is what a rollback would return it to. |

### `previous_versions`

| Name | Type | Description |
|------|------|-------------|
| `endpoint_id` | string | Identifier of the environment. |
| `version` | string | Agent version it ran before the update. |

Portainer answers with a map of environment to version; the provider sorts it into a list so the output is stable between two identical plans.
