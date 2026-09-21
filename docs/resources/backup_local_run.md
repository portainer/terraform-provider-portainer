# Resource Documentation: `portainer_backup_local_run`

# portainer_backup_local_run

> **Business Edition only.** This endpoint does not exist in Portainer CE.


Runs a local backup once, outside the schedule.

## Example Usage

```hcl
resource "portainer_backup_local_run" "before_upgrade" {
  triggers = {
    version = var.portainer_version
  }
}
```

## Arguments Reference

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `triggers` | map | ❌ no | Arbitrary values that force another run when they change. |

## Attributes Reference

| Name | Type | Description |
|------|------|-------------|
| `path` | string | Path on the Portainer host where the archive was written. |
| `failed` | bool | Whether Portainer reports the run as failed. |
| `timestamp` | string | UTC timestamp Portainer reports for the run. |

Without a `triggers` value the resource runs once and then stays put, since a backup is a one-shot action.
