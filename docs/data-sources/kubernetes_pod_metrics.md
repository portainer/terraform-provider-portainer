# Data Source Documentation: `portainer_kubernetes_pod_metrics`

# portainer_kubernetes_pod_metrics
Reads live pod resource usage from metrics-server, for one namespace or a single pod.

## Example Usage

```hcl
data "portainer_kubernetes_pod_metrics" "prod" {
  environment_id = 1
  namespace      = "prod"
}

output "pod_cpu" {
  value = {
    for p in data.portainer_kubernetes_pod_metrics.prod.pods :
    p.name => join(", ", [for c in p.containers : "${c.name}=${c.cpu}"])
  }
}
```

## Arguments Reference

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `environment_id` | number | ✅ yes | Environment (endpoint) identifier of the Kubernetes environment. |
| `namespace` | string | ✅ yes | Namespace whose pod metrics are read. |
| `pod` | string | ❌ no | Single pod to read. Leave unset for every pod in the namespace. |

## Attributes Reference

| Name | Type | Description |
|------|------|-------------|
| `pods` | list | Pod metrics. Each entry has `name`, `timestamp`, `window` and `containers` (each with `name`, `cpu`, `memory`). |

Requires metrics-server in the cluster; without it the endpoint fails and so does this data source. A single pod is still reported as a one-element list, so consumers do not have to branch on which form was requested.
