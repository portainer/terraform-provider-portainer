# Data Source Documentation: `portainer_gitops_source`

# portainer_gitops_source
The `portainer_gitops_source` data source reads a single GitOps source by ID, including its access control and the workflows deployed from it.

## Example Usage

```hcl
data "portainer_gitops_source" "infra" {
  source_id = 12
}

# A source cannot be deleted while a workflow still uses it.
output "infra_workflows" {
  value = [for w in data.portainer_gitops_source.infra.workflows : w.name]
}
```

## Arguments Reference

| Name        | Type   | Required | Description                                    |
|-------------|--------|----------|------------------------------------------------|
| `source_id` | number | ✅ yes   | Identifier of the GitOps source to look up.     |

## Attributes Reference

| Name              | Type   | Description                                                                                                                                                     |
|-------------------|--------|-------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `name`            | string | Name of the source.                                                                                                                                               |
| `url`             | string | Repository URL the source syncs from.                                                                                                                             |
| `type`            | string | Source type: `git`, `helm` or `oci`.                                                                                                                              |
| `status`          | string | Sync status: `healthy`, `syncing`, `error`, `paused` or `unknown`.                                                                                                |
| `status_error`    | string | Error reported by the last synchronisation attempt, empty while healthy.                                                                                          |
| `interval`        | string | Polling interval of the source.                                                                                                                                   |
| `last_sync`       | number | Unix timestamp of the last successful synchronisation.                                                                                                            |
| `username`        | string | Username the source authenticates with. Portainer never returns the password.                                                                                     |
| `tls_skip_verify` | bool   | Whether TLS certificate verification is skipped.                                                                                                                  |
| `public`          | bool   | Whether every user can use this source.                                                                                                                           |
| `user_accesses`   | list   | IDs of users granted access.                                                                                                                                      |
| `team_accesses`   | list   | IDs of teams granted access.                                                                                                                                      |
| `workflows`       | list   | Workflows deployed from this source. Each entry has `id`, `name`, `type`, `platform`, `namespace`, `endpoint_id`, `edge_group_ids`, `creation_date`, `last_sync_date`. |
