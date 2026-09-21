# Data Source Documentation: `portainer_environment_metrics`

# portainer_environment_metrics

> **Business Edition only.** This endpoint does not exist in Portainer CE.


Aggregates a metric over a time window for an environment.

## Example Usage

```hcl
data "portainer_environment_metrics" "cpu" {
  endpoint_id = portainer_environment.prod.id
  metric      = "cpu"
  aggregation = "avg"
  group_by    = "namespace"
  from        = "2026-01-01T00:00:00Z"
  to          = "2026-01-02T00:00:00Z"
}

output "cpu_by_namespace" {
  value = jsondecode(data.portainer_environment_metrics.cpu.data)
}
```

## Arguments Reference

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `aggregation` | string | ✅ yes | How to aggregate it, for example `avg` or `max`. |
| `endpoint_id` | number | ✅ yes | Identifier of the environment to query metrics for. |
| `from` | string | ✅ yes | Start of the window to query. |
| `group_by` | string | ❌ no | Dimension to group the result by. |
| `kind` | string | ❌ no | Only aggregate over this resource kind. |
| `metric` | string | ✅ yes | Metric to aggregate. |
| `name` | string | ❌ no | Only aggregate over the resource with this name. |
| `namespace` | string | ❌ no | Only aggregate over this namespace. |
| `to` | string | ✅ yes | End of the window to query. |

## Attributes Reference

| Name | Type | Description |
|------|------|-------------|
| `data` | string | The aggregated series as Portainer returns them, encoded as JSON. Decode it with `jsondecode()`; the shape depends on `metric` and `group_by`. |

The shape of the returned series depends on `metric` and `group_by`, so `data` is carried through as JSON rather than flattened into attributes that would only fit one kind of query. A window with no data reads as `[]`.
