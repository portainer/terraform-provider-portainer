# Resource Documentation: `portainer_alerting_silence`

# portainer_alerting_silence
The `portainer_alerting_silence` resource creates and manages alert silences in Portainer. A silence suppresses alerts matching specified label matchers for a given time window.

All fields are immutable (`ForceNew`). Changing any field will destroy the existing silence and create a new one.

## Example Usage

### Silence all critical alerts for maintenance
```hcl
resource "portainer_alerting_silence" "maintenance" {
  alert_manager_url = "http://alertmanager.monitoring.svc:9093"
  comment           = "Scheduled maintenance window"
  created_by        = "terraform"
  starts_at         = "2026-04-12T00:00:00Z"
  ends_at           = "2026-04-12T06:00:00Z"

  matchers {
    name     = "severity"
    value    = "critical"
    is_regex = false
    is_equal = true
  }
}
```

### Silence alerts matching a regex pattern
```hcl
resource "portainer_alerting_silence" "regex_silence" {
  alert_manager_url = "http://alertmanager.monitoring.svc:9093"
  comment           = "Suppress disk alerts on test nodes"
  created_by        = "terraform"
  starts_at         = "2026-04-12T00:00:00Z"
  ends_at           = "2026-04-13T00:00:00Z"

  matchers {
    name     = "alertname"
    value    = "DiskSpace.*"
    is_regex = true
    is_equal = true
  }

  matchers {
    name     = "environment"
    value    = "test"
    is_regex = false
    is_equal = true
  }
}
```

## Lifecycle & Behavior
- **Create**: Sends `POST /observability/alerting/silence` with the silence definition.
- **Read**: Verifies the silence exists by querying `GET /observability/alerting/alerts?status=silenced`.
- **Delete**: Sends `DELETE /observability/alerting/silence/{id}?alertManagerURL=<url>`.
- **Update**: Not supported. All fields force recreation.

## Arguments Reference

### Main Attributes
| Name                | Type   | Required | Description                                            |
|---------------------|--------|----------|--------------------------------------------------------|
| `alert_manager_url` | string | Yes      | URL of the AlertManager instance.                      |
| `comment`           | string | Yes      | Comment explaining the reason for the silence.         |
| `created_by`        | string | Yes      | Name of the user creating the silence.                 |
| `starts_at`         | string | Yes      | Start time of the silence in RFC3339 format.           |
| `ends_at`           | string | Yes      | End time of the silence in RFC3339 format.             |

### `matchers` Block (list, required)
| Name       | Type   | Required | Description                                                    |
|------------|--------|----------|----------------------------------------------------------------|
| `name`     | string | Yes      | Label name to match.                                           |
| `value`    | string | Yes      | Label value to match.                                          |
| `is_regex` | bool   | Yes      | Whether the value is a regular expression.                     |
| `is_equal` | bool   | No       | Whether to match for equality (default: `true`) or inequality. |

## Attributes Reference
| Name | Type   | Description                                 |
|------|--------|---------------------------------------------|
| `id` | string | The silence ID returned by AlertManager.    |

## Import
Silences can be imported using the silence ID:
```bash
terraform import portainer_alerting_silence.maintenance <silence-id>
```
