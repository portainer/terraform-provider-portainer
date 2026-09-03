# Data Source Documentation: `portainer_edge_job_tasks`

# portainer_edge_job_tasks
Lists the tasks of an edge job — one per environment it runs on — and whether each one's logs have been collected.

## Example Usage

```hcl
data "portainer_edge_job_tasks" "nightly" {
  edge_job_id = portainer_edge_job.nightly.id
}

output "tasks_with_logs" {
  value = [
    for t in data.portainer_edge_job_tasks.nightly.tasks : t.endpoint_name
    if t.logs_collected
  ]
}
```

## Arguments Reference

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `edge_job_id` | number | ✅ yes | Identifier of the edge job whose tasks are listed. |

## Attributes Reference

| Name | Type | Description |
|------|------|-------------|
| `tasks` | list | One task per target environment. Each entry has `id`, `endpoint_id`, `endpoint_name`, `logs_status` and `logs_collected`. |

`logs_status` follows Portainer's enum: 1 = idle, 2 = pending, 3 = collected. `logs_collected` is the last of those, and is the point at which `portainer_edge_job_task_logs` has something to return.
