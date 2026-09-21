# Data Source Documentation: `portainer_kubernetes_cron_jobs`

# portainer_kubernetes_cron_jobs

Lists the cron jobs in a cluster.

## Example Usage

```hcl
data "portainer_kubernetes_cron_jobs" "all" {
  endpoint_id = portainer_environment.prod.id
}

output "suspended_cron_jobs" {
  value = [for j in data.portainer_kubernetes_cron_jobs.all.cron_jobs : j.name if j.suspend]
}
```

## Arguments Reference

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `endpoint_id` | number | ✅ yes | Identifier of the Kubernetes environment to query. |
| `include_system` | bool | ❌ no | Whether to include Kubernetes' own system cron jobs. They are filtered out by default because they are not something a configuration acts on. |

## Attributes Reference

| Name | Type | Description |
|------|------|-------------|
| `cron_jobs` | list(object) | The cron jobs in the cluster. |

### `cron_jobs`

| Name | Type | Description |
|------|------|-------------|
| `command` | string | Command the job runs. |
| `id` | string | Kubernetes UID of the cron job. |
| `is_system` | bool | Whether this is one of Kubernetes' own system cron jobs. |
| `name` | string | Name of the cron job. |
| `namespace` | string | Namespace of the cron job. |
| `schedule` | string | Cron expression the job runs on. |
| `suspend` | bool | Whether the cron job is suspended. A suspended job does not fire. |
| `timezone` | string | Timezone the schedule is evaluated in, empty when it follows the cluster's. |

Kubernetes' own system cron jobs are filtered out unless `include_system` is set, because they are not something a configuration acts on.
