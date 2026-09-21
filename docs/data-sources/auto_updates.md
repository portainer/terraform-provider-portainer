# Data Source Documentation: `portainer_auto_updates`

# portainer_auto_updates

> **Business Edition only.** This endpoint does not exist in Portainer CE.


Lists the automatic update runs Portainer has recorded for itself.

This is the history, not the schedule: what triggers the updates lives in Portainer's auto-patch settings.

## Example Usage

```hcl
data "portainer_auto_updates" "history" {}

output "auto_update_in_progress" {
  value = length([
    for u in data.portainer_auto_updates.history.auto_updates : u
    if u.status == "inProgress"
  ]) > 0
}
```

## Attributes Reference

| Name | Type | Description |
|------|------|-------------|
| `auto_updates` | list(object) | The automatic update runs Portainer has recorded. |

### `auto_updates`

| Name | Type | Description |
|------|------|-------------|
| `version` | string | Portainer version the run updated to. |
| `status` | string | Outcome of the run: `inProgress`, `completed` or `failed`. |
| `started_at` | number | Unix timestamp the run started at. |
| `done_at` | number | Unix timestamp the run finished at, zero while it is still running. |

Portainer does not document an ordering for the list, so sort on `started_at` rather than assuming the newest run is first or last.
