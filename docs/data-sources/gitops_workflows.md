# Data Source Documentation: `portainer_gitops_workflows`

# portainer_gitops_workflows
The `portainer_gitops_workflows` data source lists Portainer's GitOps workflows with optional filtering and paging, together with the status summary.

A workflow has three phases — source, artifact and target — each with its own status. The `healthy` attribute is true only when all three report `healthy`, which is what Portainer's UI shows as a green workflow.

## Example Usage

```hcl
data "portainer_gitops_workflows" "all" {
  platform     = "kubernetes"
  endpoint_ids = [3, 4]
}

output "failing_workflows" {
  value = [
    for w in data.portainer_gitops_workflows.all.workflows :
    "${w.name}: source=${w.source_status} artifact=${w.artifact_status} target=${w.target_status}"
    if !w.healthy
  ]
}
```

## Arguments Reference

| Name           | Type   | Required | Description                                                                             |
|----------------|--------|----------|-----------------------------------------------------------------------------------------|
| `search`       | string | ❌ no    | Free-text filter matched against the workflow name.                                      |
| `sort`         | string | ❌ no    | Field to sort by, as accepted by the Portainer API.                                       |
| `order`        | string | ❌ no    | Sort direction, `asc` or `desc`.                                                          |
| `start`        | number | ❌ no    | Zero-based index of the first result, for paging.                                         |
| `limit`        | number | ❌ no    | Maximum number of results to return.                                                      |
| `endpoint_ids` | list   | ❌ no    | Only return workflows deploying to these environment identifiers.                         |
| `status`       | string | ❌ no    | Only return workflows in this status.                                                     |
| `type`         | string | ❌ no    | Only return workflows of this type: `stack` or `edgeStack`.                               |
| `platform`     | string | ❌ no    | Only return workflows for this platform: `dockerStandalone`, `dockerSwarm`, `kubernetes`. |

## Attributes Reference

| Name        | Type | Description                                                                                                                                                                            |
|-------------|------|--------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `workflows` | list | Workflows matching the query. Each entry has `id`, `name`, `source_status`, `source_error`, `artifact_status`, `artifact_error`, `target_status`, `target_error`, `healthy`, `creation_date` and `last_sync_date`. |
| `summary`   | list | Exactly one element with the counts `healthy`, `syncing`, `error`, `paused` and `unknown`.                                                                                              |

The summary comes from its own Portainer endpoint and is not filtered by the arguments above.
