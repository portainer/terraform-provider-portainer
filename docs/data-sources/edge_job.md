# ⏰ **Data Source Documentation: `portainer_edge_job`**

# portainer_edge_job
The `portainer_edge_job` data source allows you to look up an existing Portainer Edge job by its name.

## Example Usage

### Look up an Edge job by name

```hcl
data "portainer_edge_job" "backup" {
  name = "nightly-backup"
}

output "job_id" {
  value = data.portainer_edge_job.backup.id
}
```

## Arguments Reference

| Name   | Type   | Required | Description              |
|--------|--------|----------|--------------------------|
| `name` | string | ✅ yes   | Name of the Portainer Edge job. |

## Attributes Reference

| Name              | Type   | Description                         |
|-------------------|--------|-------------------------------------|
| `id`              | string | ID of the Portainer Edge job.       |
| `cron_expression` | string | The cron schedule for the job.      |
