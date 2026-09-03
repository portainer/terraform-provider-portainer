# Data Source Documentation: `portainer_gitops_workflow`

# portainer_gitops_workflow
The `portainer_gitops_workflow` data source reads a single GitOps workflow by ID, with the status of each of its three phases.

## Example Usage

```hcl
data "portainer_gitops_workflow" "web" {
  workflow_id = 5
}

# Block a downstream change until the workflow has converged.
resource "portainer_kubernetes_deployment_scale" "web" {
  count = data.portainer_gitops_workflow.web.healthy ? 1 : 0

  environment_id  = 1
  namespace       = "prod"
  deployment_name = "web"
  replicas        = 3
}
```

## Arguments Reference

| Name          | Type   | Required | Description                                      |
|---------------|--------|----------|--------------------------------------------------|
| `workflow_id` | number | ✅ yes   | Identifier of the GitOps workflow to look up.     |

## Attributes Reference

| Name              | Type   | Description                                                                            |
|-------------------|--------|----------------------------------------------------------------------------------------|
| `name`            | string | Name of the workflow.                                                                    |
| `source_status`   | string | Status of the source phase: `healthy`, `syncing`, `error`, `paused` or `unknown`.        |
| `source_error`    | string | Error reported by the source phase, empty while healthy.                                 |
| `artifact_status` | string | Status of the artifact phase.                                                            |
| `artifact_error`  | string | Error reported by the artifact phase.                                                    |
| `target_status`   | string | Status of the target (deployment) phase.                                                 |
| `target_error`    | string | Error reported by the target phase.                                                      |
| `healthy`         | bool   | Whether all three phases report `healthy`.                                               |
| `creation_date`   | number | Unix timestamp at which the workflow was created.                                        |
| `last_sync_date`  | number | Unix timestamp of the workflow's last synchronisation.                                   |
