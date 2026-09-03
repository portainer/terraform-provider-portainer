# Data Source Documentation: `portainer_edge_job_task_logs`

# portainer_edge_job_task_logs
Reads the collected log output of a single edge job task.

## Example Usage

```hcl
data "portainer_edge_job_task_logs" "nightly" {
  edge_job_id = portainer_edge_job.nightly.id
  task_id     = data.portainer_edge_job_tasks.nightly.tasks[0].id
}

output "nightly_output" {
  value = data.portainer_edge_job_task_logs.nightly.logs
}
```

## Arguments Reference

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `edge_job_id` | number | ✅ yes | Identifier of the edge job. |
| `task_id` | string | ✅ yes | Identifier of the task, from the `portainer_edge_job_tasks` data source. |

## Attributes Reference

| Name | Type | Description |
|------|------|-------------|
| `logs` | string | Collected log output of the task, empty when nothing has been collected yet. |

Logs only exist after collection has been requested with the `portainer_edge_job_task_logs` **resource** and the edge agent has checked in and uploaded them. Until then this returns an empty string rather than failing, so a plan does not break on a job that has not reported yet.
