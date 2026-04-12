# Data Source Documentation: `portainer_user_activity`

# portainer_user_activity
The `portainer_user_activity` data source allows you to retrieve user activity logs or authentication logs from Portainer.

## Example Usage

### List recent user activity logs

```hcl
data "portainer_user_activity" "recent" {
  log_type = "activity"
  limit    = 50
}

output "activity_logs" {
  value = data.portainer_user_activity.recent.activity_logs
}
```

### List authentication logs filtered by keyword

```hcl
data "portainer_user_activity" "auth" {
  log_type = "auth"
  keyword  = "admin"
  limit    = 20
}

output "auth_logs" {
  value = data.portainer_user_activity.auth.auth_logs
}
```

### Filter activity logs by username and date range

```hcl
data "portainer_user_activity" "filtered" {
  log_type  = "activity"
  username  = ["admin"]
  after     = 1700000000
  before    = 1710000000
  sort_by   = "timestamp"
  sort_desc = true
}
```

## Arguments Reference

| Name        | Type         | Required | Description                                                                 |
|-------------|--------------|----------|-----------------------------------------------------------------------------|
| `log_type`  | string       | No       | Type of logs: `activity` (default) or `auth`.                               |
| `keyword`   | string       | No       | Filter logs by keyword.                                                     |
| `username`  | list(string) | No       | Filter by usernames (activity logs only).                                   |
| `context`   | list(string) | No       | Filter by context (activity logs only).                                     |
| `before`    | number       | No       | Return results before this unix timestamp.                                  |
| `after`     | number       | No       | Return results after this unix timestamp.                                   |
| `sort_by`   | string       | No       | Column to sort by.                                                          |
| `sort_desc` | bool         | No       | Sort in descending order (default: false).                                  |
| `offset`    | number       | No       | Pagination offset (default: 0).                                            |
| `limit`     | number       | No       | Maximum number of results (default: 100).                                   |

## Attributes Reference

| Name            | Type | Description                                              |
|-----------------|------|----------------------------------------------------------|
| `activity_logs` | list | List of user activity logs (populated when log_type is `activity`). Each entry has `id`, `timestamp`, `username`, `action`, `context`. |
| `auth_logs`     | list | List of auth activity logs (populated when log_type is `auth`). Each entry has `id`, `timestamp`, `username`, `type`, `origin`, `context`. |
| `total_count`   | number | Total count of matching activity logs.                  |
