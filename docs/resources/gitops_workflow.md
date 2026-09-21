# Resource Documentation: `portainer_gitops_workflow`

# portainer_gitops_workflow

> **Business Edition.** Portainer CE has the workflow create, read and update endpoints, but
> neither of the two removal endpoints this resource destroys through - so the resource as a
> whole needs Business Edition.


Manages a GitOps workflow: a named set of artifacts, each built from files in a GitOps source and deployed onto a set of edge groups.

## Example Usage

```hcl
resource "portainer_gitops_workflow" "platform" {
  name       = "platform"
  on_destroy = "destroy"

  artifact {
    name            = "monitoring"
    type            = "edgeStack"
    deployment_type = "compose"
    edge_group_ids  = [portainer_edge_group.shops.id]

    file {
      source_id = portainer_gitops_source.platform.id
      path      = "monitoring/portainer.yaml"
      ref       = "refs/heads/main"
    }

    config {
      pre_pull_image = true
      registry_ids   = [portainer_registry.internal.id]

      environment = {
        LOG_LEVEL = "info"
      }

      parallel {
        batch_count    = 5
        delay          = "30"
        failure_action = "continue"
      }
    }
  }
}
```

## Arguments Reference

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `name` | string | ✅ yes | Name of the workflow. |
| `on_destroy` | string | ❌ no | What happens to the deployed artifacts on destroy: `destroy` or `detach`. Defaults to `destroy`. |
| `artifact` | block | ✅ yes | The artifacts the workflow deploys. Repeatable. |

### `artifact`

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `name` | string | ✅ yes | Name of the artifact. |
| `type` | string | ❌ no | `stack` or `edgeStack`. Defaults to `edgeStack`. |
| `deployment_type` | string | ✅ yes | Deployment type, for example `compose` or `kubernetes`. |
| `edge_group_ids` | list(number) | ✅ yes | Edge groups the artifact is deployed to. |
| `file` | block | ✅ yes | Files the artifact is built from. Repeatable. |
| `config` | block | ❌ no | Deployment options. At most one. |

#### `artifact.file`

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `source_id` | number | ✅ yes | GitOps source the file comes from. |
| `path` | string | ✅ yes | Path within the source. |
| `ref` | string | ✅ yes | Git reference to read the file at. |

#### `artifact.config`

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `environment` | map(string) | ❌ no | Environment variables injected into the deployment. |
| `registry_ids` | list(number) | ❌ no | Registries the deployment pulls images from. |
| `pre_pull_image` | bool | ❌ no | Whether agents pull images before deploying. |
| `retry_period` | number | ❌ no | How long an agent keeps retrying a failed deployment, in seconds. |
| `use_manifest_namespaces` | bool | ❌ no | Use the namespaces in the manifest rather than the default one. |
| `always_clone_git_repo` | bool | ❌ no | Always clone the git repository for relative paths. |
| `local_filesystem_path` | string | ❌ no | Path on the agent used for relative path volumes. |
| `per_device_configs_path` | string | ❌ no | Path holding per-device configurations. |
| `per_device_configs_match_type` | string | ❌ no | `file` or `dir`. |
| `per_device_configs_group_match_type` | string | ❌ no | `file` or `dir`. |
| `parallel` | block | ❌ no | Staggered rollout settings: `batch_count`, `batch_increment_by`, `delay`, `timeout`, `failure_action`. At most one. |

## Attributes Reference

| Name | Type | Description |
|------|------|-------------|
| `workflow_id` | number | Identifier of the workflow. |
| `artifact.artifact_id` | number | Identifier Portainer assigned to the artifact, which the update call needs. |

## Lifecycle & Behavior

**`on_destroy` is a real choice.** Portainer has two removal endpoints that do different things: `destroy` tears the deployed stacks down along with the workflow, `detach` leaves them running and only unlinks them from GitOps. The default is `destroy`, which is what removing a resource normally means in Terraform.

**Only artifact identifiers are read back.** The update payload keys artifacts by the identifier Portainer assigned, so those are read into state. The rest of an artifact is driven by the configuration: Portainer normalises the file and config fields, and reading them back would make a stable configuration churn.

## Import

```bash
terraform import portainer_gitops_workflow.platform 4
```

The import ID is the workflow identifier. An imported workflow has no artifact blocks in state, so they have to be supplied in the configuration before the first apply.
