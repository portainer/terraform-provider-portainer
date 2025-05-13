# 🧭 **Resource Documentation: `portainer_edge_job`**

# portainer_edge_job
The `portainer_edge_job` resource allows you to create and schedule Edge Jobs in Portainer.
You can use either inline `file_content` or upload a script via `file_path`.

## Example Usage
### Create Edge Job using file_content
```hcl
resource "portainer_edge_group" "static_group" {
  name    = "static-group"
  dynamic = false
}

resource "portainer_edge_job" "string_job" {
  name            = "job-from-string"
  cron_expression = "0 * * * *"
  edge_groups     = [portainer_edge_group.static_group.id]
  endpoints       = [2]
  recurring       = true
  file_content = <<-EOT
    echo "Hello from string job!"
  EOT
}
```

### Create Edge Job from a script file
```hcl
resource "portainer_edge_group" "static_group" {
  name    = "static-group"
  dynamic = false
}

resource "portainer_edge_job" "file_job" {
  name            = "job-from-file"
  cron_expression = "0 12 * * *"
  edge_groups     = edge_groups     = [portainer_edge_group.static_group.id]
  endpoints       = [2]
  recurring       = false
  file_path       = "scripts/my-job.sh"
}
```
## Lifecycle & Behavior
Edge jobs are always re-applied when Terraform is run, as Portainer treats them as triggered actions.
- To delete an edge group created via Terraform, simply run:
```hcl
terraform destroy
```

- To re-run with new content, change the script or cron and apply:
```hcl
terraform apply
```
> ⚠️ You must provide either file_content or file_path – not both. ⚠️ If recurring = false, the job runs once immediately.

## Arguments Reference

### Common Arguments
| Name              | Type       | Required      | Description                                                                |
|-------------------|------------|---------------|----------------------------------------------------------------------------|
| `name`            | string     | ✅ yes        | Name of the Edge Job                                                       |
| `cron_expression` | string     | ✅ yes        | Cron expression for job scheduling (e.g. `0 * * * *`)                      |
| `edge_groups`     | list(int)  | ✅ yes        | List of Edge Group IDs where the job will run                              |
| `endpoints`       | list(int)  | ✅ yes        | List of specific environment IDs where job will run                        |
| `recurring`       | bool       | 🚫 optional   | Whether the job should repeat based on cron expression (default: `true`)   |

### For String-based Job Creation
| Name           | Type   | Required    | Description                                                  |
|----------------|--------|-------------|--------------------------------------------------------------|
| `file_content` | string | ✅ required | Inline shell script content (mutually exclusive with `file_path`) |

### For File-based Job Creation
| Name       | Type   | Required    | Description                                               |
|------------|--------|-------------|-----------------------------------------------------------|
| `file_path`| string | ✅ required | Path to local script file (mutually exclusive with `file_content`) |

## Attributes Reference

| Name | Description              |
|------|--------------------------|
| `id` | ID of the Portainer edge job |
