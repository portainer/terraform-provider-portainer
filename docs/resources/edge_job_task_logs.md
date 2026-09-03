# Resource Documentation: `portainer_edge_job_task_logs`

# portainer_edge_job_task_logs
The `portainer_edge_job_task_logs` resource asks Portainer to collect the log output of one edge job task, and clears it again when destroyed.

Collection is **asynchronous**: creating this resource only registers the request. The edge agent uploads the logs on its next check-in, so they are not readable the moment the apply returns. Read them afterwards with the `portainer_edge_job_task_logs` data source, and use `logs_collected` on `portainer_edge_job_tasks` to tell whether they have arrived.

## Example Usage

```hcl
data "portainer_edge_job_tasks" "nightly" {
  edge_job_id = portainer_edge_job.nightly.id
}

resource "portainer_edge_job_task_logs" "collect" {
  edge_job_id = portainer_edge_job.nightly.id
  task_id     = data.portainer_edge_job_tasks.nightly.tasks[0].id
}
```

## Arguments Reference

| Name          | Type   | Required | Description                                                                        |
|---------------|--------|----------|------------------------------------------------------------------------------------|
| `edge_job_id` | number | ✅ yes   | Identifier of the edge job. Changing it replaces the resource.                      |
| `task_id`     | string | ✅ yes   | Identifier of the task, from `portainer_edge_job_tasks`. Changing it replaces it.   |

## Attributes Reference

| Name | Description                                                          |
|------|----------------------------------------------------------------------|
| `id` | Composite identifier in the form `<edge_job_id>/<task_id>/logs`.      |

Destroying the resource clears the collected logs on the Portainer side. Logs that were never there are treated as already cleared.
