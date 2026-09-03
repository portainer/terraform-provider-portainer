# Data Source Documentation: `portainer_kubernetes_application_resources`

# portainer_kubernetes_application_resources
Totals what the cluster's applications request and are limited to, which is what you compare against the cluster's capacity before scheduling more.

## Example Usage

```hcl
data "portainer_kubernetes_application_resources" "prod" {
  environment_id = 1
}

data "portainer_kubernetes_cluster" "prod" {
  environment_id = 1
}

output "cpu_committed_vs_headroom" {
  value = "${data.portainer_kubernetes_application_resources.prod.cpu_request} cores requested, ${data.portainer_kubernetes_cluster.prod.max_cpu}m still schedulable"
}
```

## Arguments Reference

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `environment_id` | number | ✅ yes | Environment (endpoint) identifier of the Kubernetes environment. |

## Attributes Reference

| Name | Type | Description |
|------|------|-------------|
| `cpu_request` | number | Total CPU requested by the applications, in cores. |
| `cpu_limit` | number | Total CPU limit across the applications, in cores. |
| `memory_request` | number | Total memory requested, in bytes. |
| `memory_limit` | number | Total memory limit, in bytes. |

CPU is reported in cores as a fractional number, while `portainer_kubernetes_cluster` reports its headroom in millicores — do not compare the two without converting.
