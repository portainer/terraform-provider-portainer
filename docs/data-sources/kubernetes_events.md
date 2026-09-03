# Data Source Documentation: `portainer_kubernetes_events`

# portainer_kubernetes_events
Reads Kubernetes events, cluster-wide or for one namespace. Events are usually what explains a workload that will not start.

## Example Usage

```hcl
data "portainer_kubernetes_events" "prod" {
  environment_id = 1
  namespace      = "prod"
}

output "problems" {
  value = [
    for e in data.portainer_kubernetes_events.prod.events : "${e.kind}/${e.name}: ${e.reason} - ${e.message}"
    if e.type == "Warning"
  ]
}
```

## Arguments Reference

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `environment_id` | number | ✅ yes | Environment (endpoint) identifier of the Kubernetes environment. |
| `namespace` | string | ❌ no | Namespace to read events from. Leave unset for every accessible namespace. |
| `resource_id` | string | ❌ no | Only return events for the object with this UID. Cluster-wide queries only. |

## Attributes Reference

| Name | Type | Description |
|------|------|-------------|
| `events` | list | Events matching the query. Each entry has `type`, `reason`, `message`, `count`, `namespace`, `kind`, `name`, `uid`, `first_timestamp` and `last_timestamp`. |
| `warning_count` | number | Number of returned events of type `Warning`. |

Kubernetes discards events after about an hour by default, so this describes the recent past only — an empty list is not evidence that nothing went wrong.
