# Data Source Documentation: `portainer_kubernetes_node_metrics`

# portainer_kubernetes_node_metrics
Reads live resource usage of a single Kubernetes node from metrics-server.

## Example Usage

```hcl
data "portainer_kubernetes_node_metrics" "worker" {
  environment_id = 1
  node           = "worker-1"
}

output "worker_cpu" {
  value = data.portainer_kubernetes_node_metrics.worker.cpu
}
```

## Arguments Reference

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `environment_id` | number | ✅ yes | Environment (endpoint) identifier of the Kubernetes environment. |
| `node` | string | ✅ yes | Name of the node to read metrics for. |

## Attributes Reference

| Name | Type | Description |
|------|------|-------------|
| `cpu` | string | CPU usage as a Kubernetes quantity string. |
| `memory` | string | Memory usage as a Kubernetes quantity string. |
| `timestamp` | string | When the sample was taken. |
| `window` | string | Length of the sampling window. |

Use `portainer_kubernetes_nodes` to get usage for every node at once — it already folds the cluster-wide metrics endpoint in, and tolerates a cluster without metrics-server.
