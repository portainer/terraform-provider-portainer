# Data Source Documentation: `portainer_edge_update_schedules_active`

# portainer_edge_update_schedules_active

> **Business Edition only.** This endpoint does not exist in Portainer CE.


Reports which update schedules are currently in flight for a set of environments.

## Example Usage

```hcl
data "portainer_edge_update_schedules_active" "shops" {
  environment_ids = [12, 13]
}

output "updates_in_flight" {
  value = length(data.portainer_edge_update_schedules_active.shops.schedules) > 0
}
```

## Arguments Reference

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `environment_ids` | list(string) | ✅ yes | Environments to report active update schedules for. |

## Attributes Reference

| Name | Type | Description |
|------|------|-------------|
| `schedules` | list(object) | The update schedules currently in flight. An empty list means none of the given environments is mid-update. |

### `schedules`

| Name | Type | Description |
|------|------|-------------|
| `edge_stack_id` | number | Edge stack carrying out the update. |
| `endpoint_id` | number | Environment the schedule is updating. |
| `schedule_id` | number | Identifier of the update schedule. |
| `target_version` | string | Agent version the schedule is moving the environment to. |

Portainer takes the environment list in a POST body, but the call changes nothing - which is why this is a data source. An empty `schedules` means none of the given environments is mid-update.
