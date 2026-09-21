# Data Source Documentation: `portainer_environment_logs`

# portainer_environment_logs

> **Business Edition only.** This endpoint does not exist in Portainer CE.


Queries resource-scoped logs for an environment over a time window.

## Example Usage

```hcl
data "portainer_environment_logs" "errors" {
  endpoint_id = portainer_environment.prod.id
  from        = "2026-01-01T00:00:00Z"
  to          = "2026-01-02T00:00:00Z"
  namespace   = "apps"
  severity    = "error"
  limit       = 100
}
```

## Arguments Reference

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `endpoint_id` | number | ✅ yes | Identifier of the environment to query logs for. |
| `from` | string | ✅ yes | Start of the window to query, as Portainer's observability backend expects it (for example an RFC 3339 timestamp). |
| `kind` | string | ❌ no | Only return logs from this resource kind. |
| `limit` | number | ❌ no | Most log lines to return. |
| `name` | string | ❌ no | Only return logs from the resource with this name. |
| `namespace` | string | ❌ no | Only return logs from this namespace. |
| `search` | string | ❌ no | Only return log lines matching this text. |
| `severity` | string | ❌ no | Only return log lines of this severity. |
| `skip` | number | ❌ no | Log lines to skip, for paging through a large window. |
| `to` | string | ✅ yes | End of the window to query. |

## Attributes Reference

| Name | Type | Description |
|------|------|-------------|
| `logs` | list(object) | The log lines the query returned. |

### `logs`

| Name | Type | Description |
|------|------|-------------|
| `labels` | map(string) | Labels attached to the line. |
| `message` | string | The log line itself. |
| `severity` | string | Severity of the line. |
| `source` | string | Where the line came from. |
| `time` | string | When the line was logged. |

`from` and `to` are required: this queries a window, not a tail. The filters that are left unset are omitted from the request rather than sent empty.

Logs change between reads, so a configuration that depends on this value will show a difference on every plan. Use it for an output or a local file, not as an input to another resource.
