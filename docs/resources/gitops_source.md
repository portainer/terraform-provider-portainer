# Resource Documentation: `portainer_gitops_source`

# portainer_gitops_source
The `portainer_gitops_source` resource manages a Git source in Portainer's GitOps feature. A source is the repository Portainer polls; workflows then deploy artifacts from it.

## Example Usage

### Public repository

```hcl
resource "portainer_gitops_source" "infra" {
  url      = "https://github.com/acme/infra.git"
  name     = "infra"
  interval = "5m"
  public   = true
}
```

### Private repository with access control

```hcl
resource "portainer_gitops_source" "apps" {
  url      = "https://github.com/acme/apps.git"
  name     = "apps"
  interval = "10m"

  username = "ci-bot"
  password = var.git_token

  public        = false
  user_accesses = [portainer_user.deployer.id]
  team_accesses = [portainer_team.platform.id]
}
```

Validate the credentials before creating the source with the `portainer_gitops_source_connection` data source.

## Arguments Reference

| Name                  | Type   | Required | Default | Description                                                                                                                       |
|-----------------------|--------|----------|---------|-------------------------------------------------------------------------------------------------------------------------------------|
| `url`                 | string | ✅ yes   | –       | URL of the Git repository the source syncs from.                                                                                      |
| `name`                | string | ❌ no    | derived | Name shown in Portainer. Portainer derives one from the repository URL when unset.                                                    |
| `interval`            | string | ❌ no    | server  | Polling interval as a duration string (for example `5m`).                                                                             |
| `username`            | string | ❌ no    | –       | Username used to authenticate against the repository.                                                                                 |
| `password`            | string | ❌ no    | –       | Password or personal access token paired with `username`. Stored in state as a sensitive value.                                        |
| `tls_skip_verify`     | bool   | ❌ no    | `false` | Skip TLS certificate verification when contacting the repository.                                                                     |
| `administrators_only` | bool   | ❌ no    | `false` | Restrict the source to administrators. Only honoured at creation time — changing it replaces the resource.                             |
| `public`              | bool   | ❌ no    | server  | Whether every user can use this source.                                                                                               |
| `user_accesses`       | list   | ❌ no    | –       | IDs of users granted access to the source.                                                                                            |
| `team_accesses`       | list   | ❌ no    | –       | IDs of teams granted access to the source.                                                                                            |

Portainer splits an update across two endpoints: `url`, `name`, `interval`, `tls_skip_verify` and the credentials go to the source itself, while `public`, `user_accesses` and `team_accesses` go to its access-control endpoint. The provider calls each only when its own fields changed.

`password` is never returned by the API, so it cannot be checked for drift — a password changed in the Portainer UI stays invisible to Terraform until the value in the configuration changes.

## Attributes Reference

| Name           | Description                                                                              |
|----------------|------------------------------------------------------------------------------------------|
| `id`           | Identifier of the GitOps source.                                                          |
| `type`         | Source type reported by Portainer: `git`, `helm` or `oci`. This resource creates `git`.    |
| `status`       | Sync status: `healthy`, `syncing`, `error`, `paused` or `unknown`.                         |
| `status_error` | Error reported by the last synchronisation attempt, empty while healthy.                  |
| `last_sync`    | Unix timestamp of the last successful synchronisation.                                    |

## Import

GitOps sources can be imported using their numeric ID:

```shell
terraform import portainer_gitops_source.infra 12
```

`password` is not part of the imported state — set it in the configuration afterwards.
