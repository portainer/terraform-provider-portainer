# Resource Documentation: `portainer_alerting_settings`

# portainer_alerting_settings
The `portainer_alerting_settings` resource manages Portainer alerting (observability) settings, including enabling/disabling alerting and configuring notification channels.

This is a settings-style resource: `terraform apply` creates or updates the settings, and `terraform destroy` disables alerting.

## Example Usage

### Enable internal alerting with a Slack notification channel
```hcl
resource "portainer_alerting_settings" "main" {
  enabled       = true
  name          = "Internal AlertManager"
  portainer_url = "https://portainer.example.com"

  notification_channels {
    channel_id = 1
    name       = "ops-slack"
    type       = "slack"
    enabled    = true

    config = {
      webhookURL = "https://hooks.slack.com/services/T00/B00/xxx"
      channel    = "#alerts"
    }
  }
}
```

### Enable external AlertManager
```hcl
resource "portainer_alerting_settings" "external" {
  enabled       = true
  name          = "External AlertManager"
  url           = "http://alertmanager.monitoring.svc:9093"
  portainer_url = "https://portainer.example.com"
}
```

## Lifecycle & Behavior
- **Create/Update**: Sends `PUT /observability/alerting/settings` with the provided configuration.
- **Read**: Reads from `GET /observability/alerting/settings` and matches the entry by ID.
- **Delete**: Disables alerting by sending `enabled = false` via `PUT`.

## Arguments Reference

### Main Attributes
| Name              | Type   | Required | Description                                                       |
|-------------------|--------|----------|-------------------------------------------------------------------|
| `enabled`         | bool   | Yes      | Whether alerting is enabled.                                      |
| `name`            | string | No       | Name of the alerting settings entry.                              |
| `url`             | string | No       | URL of the external AlertManager instance. Leave empty for internal. |
| `portainer_url`   | string | No       | Portainer URL used by AlertManager for callbacks.                 |

### `notification_channels` Block (list)
| Name         | Type        | Required | Description                                                                     |
|--------------|-------------|----------|---------------------------------------------------------------------------------|
| `channel_id` | number      | No       | Notification channel identifier.                                                |
| `name`       | string      | No       | Name of the notification channel.                                               |
| `type`       | string      | No       | Type: `slack`, `webhook`, `teams`, `discord`, `email`, `pagerduty`, `opsgenie`. |
| `enabled`    | bool        | No       | Whether the notification channel is enabled.                                    |
| `config`     | map(string) | No       | Configuration key-value pairs (e.g., webhook URL, channel name).                |

## Attributes Reference (Computed)
| Name          | Type   | Description                                                  |
|---------------|--------|--------------------------------------------------------------|
| `id`          | string | The ID of the alerting settings entry in Portainer.          |
| `is_internal` | bool   | Whether this uses the internal AlertManager.                 |
| `status`      | string | Connection status: `disabled`, `connected`, `disconnected`, `error`. |
| `uptime`      | string | Uptime of the AlertManager.                                  |
| `created_at`  | string | Timestamp when the settings were created.                    |
| `created_by`  | string | User who created the settings.                               |

## Import
Alerting settings can be imported using the settings ID:
```bash
terraform import portainer_alerting_settings.main <settings-id>
```
