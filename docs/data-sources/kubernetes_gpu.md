# Data Source Documentation: `portainer_kubernetes_gpu`

# portainer_kubernetes_gpu

> **Business Edition only.** This endpoint does not exist in Portainer CE.


Reports the GPUs a Kubernetes cluster has, what is using them, and what cannot be scheduled.

## Example Usage

```hcl
data "portainer_kubernetes_gpu" "prod" {
  endpoint_id = portainer_environment.prod.id
}

output "gpu_workloads_stuck" {
  value = [
    for w in data.portainer_kubernetes_gpu.prod.workloads : w.pod_name
    if w.scheduling_issue != ""
  ]
}
```

## Arguments Reference

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `endpoint_id` | number | ✅ yes | Identifier of the Kubernetes environment to query. |

## Attributes Reference

| Name | Type | Description |
|------|------|-------------|
| `gpu_node_count` | number | Nodes that report GPUs. |
| `degraded_count` | number | GPU nodes Portainer considers degraded. |
| `operator_detected` | bool | Whether a GPU operator was found. |
| `device_plugin_detected` | bool | Whether a GPU device plugin was found. |
| `total_capacity` | map(number) | Total GPU capacity, keyed by resource name. |
| `total_allocatable` | map(number) | Total allocatable GPUs, keyed by resource name. |
| `total_allocated` | map(number) | Total allocated GPUs, keyed by resource name. |
| `nodes` | list(object) | The GPU nodes: `name`, `status`, `status_reason`, `capacity`, `allocatable`, `allocated`. |
| `workloads` | list(object) | Workloads asking for GPUs: `pod_name`, `namespace`, `node_name`, `pod_phase`, `owner_kind`, `owner_name`, `scheduling_issue`, `gpu_requests`. |

A non-zero `degraded_count`, or a workload with a non-empty `scheduling_issue`, is what is worth alerting on here.
