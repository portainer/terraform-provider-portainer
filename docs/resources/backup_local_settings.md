# Resource Documentation: `portainer_backup_local_settings`

# portainer_backup_local_settings

> **Business Edition only.** This endpoint does not exist in Portainer CE.


Manages Portainer's scheduled backup to local storage on the Portainer host, including how long archives are kept.

## Example Usage

```hcl
resource "portainer_backup_local_settings" "nightly" {
  cron_rule      = "0 3 * * *"
  retention_days = 14
}
```

## Arguments Reference

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `cron_rule` | string | ❌ no | Cron expression for the schedule. Leave unset to keep it off and back up on demand with `portainer_backup_local_run`. |
| `retention_days` | number | ❌ no | Days Portainer keeps archives before pruning. Zero keeps them indefinitely. |

## Attributes Reference

| Name | Type | Description |
|------|------|-------------|
| `last_run_failed` | bool | Whether the most recent scheduled run failed. |
| `last_run_timestamp` | string | UTC timestamp of the most recent scheduled run. |
| `last_run_path` | string | Path on the Portainer host of the most recent archive. |

Removing the resource stops managing the settings; Portainer has no endpoint to clear them.
