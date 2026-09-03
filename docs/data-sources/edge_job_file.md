# Data Source Documentation: `portainer_edge_job_file`

# portainer_edge_job_file
Reads the script an edge job runs on its target environments.

## Example Usage

```hcl
data "portainer_edge_job_file" "nightly" {
  edge_job_id = portainer_edge_job.nightly.id
}
```

## Arguments Reference

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `edge_job_id` | number | ✅ yes | Identifier of the edge job whose script is read. |

## Attributes Reference

| Name | Type | Description |
|------|------|-------------|
| `file_content` | string | Content of the stored file, exactly as Portainer holds it. |
