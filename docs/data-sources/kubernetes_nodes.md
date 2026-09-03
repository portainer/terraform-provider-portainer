# Data Source Documentation: `portainer_kubernetes_nodes`

# portainer_kubernetes_nodes
Lists the nodes of a Kubernetes cluster with their capacity, the headroom Portainer computes for them, and their current usage.

## Example Usage

```hcl
data "portainer_kubernetes_nodes" "prod" {
  environment_id = 1
}

output "cordoned_nodes" {
  value = [for n in data.portainer_kubernetes_nodes.prod.nodes : n.name if n.unschedulable]
}

output "unready_nodes" {
  value = [for n in data.portainer_kubernetes_nodes.prod.nodes : n.name if !n.ready]
}
```

## Arguments Reference

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `environment_id` | number | ✅ yes | Environment (endpoint) identifier of the Kubernetes environment. |

## Attributes Reference

| Name | Type | Description |
|------|------|-------------|
| `nodes` | list | Nodes of the cluster. Each entry has `name`, `ready`, `unschedulable`, `kubelet_version`, `os_image`, `internal_ip`, `labels`, `capacity_cpu`, `capacity_memory`, `available_cpu`, `available_memory`, `usage_cpu` and `usage_memory`. |

`available_cpu` and `available_memory` are Portainer's own view of what is still schedulable, which is not simply capacity minus usage. `usage_cpu` and `usage_memory` come from metrics-server and are empty on a cluster that does not run it — that is reported as unknown rather than failing the read. `unschedulable` is what `portainer_kubernetes_node_drain` leaves behind.
