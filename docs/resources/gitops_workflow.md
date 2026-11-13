# Resource Documentation: `portainer_gitops_workflow`

# portainer_gitops_workflow

> **Business Edition.** Portainer CE has the workflow create, read and update endpoints, but
> neither of the two removal endpoints this resource destroys through - so the resource as a
> whole needs Business Edition.


Manages a GitOps workflow: a named set of artifacts, each built from files in a GitOps source and deployed onto a set of edge groups.

On Portainer Business Edition 3.0 or later a workflow can instead run on the **reconcile engine**, which keeps Edge Kubernetes environments continuously matching the source and reports or corrects drift. It targets Edge groups, environment groups or individual environments, and can deploy an artifact as a Helm chart.

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

### Reconcile engine (Portainer BE 3.0+)

```hcl
resource "portainer_gitops_workflow" "apps" {
  name               = "apps"
  engine             = "reconcile"
  reconcile_interval = "5m"

  artifact {
    name                  = "crds"
    deployment_type       = "kubernetes"
    environment_group_ids = [portainer_endpoint_group.production.id]

    file {
      source_id = portainer_gitops_source.platform.id
      path      = "crds"
      ref       = "refs/heads/main"
    }
  }

  artifact {
    name                  = "web"
    deployment_type       = "kubernetes"
    environment_group_ids = [portainer_endpoint_group.production.id]
    dependencies          = ["crds"]

    file {
      source_id = portainer_gitops_source.platform.id
      path      = "charts/web"
      ref       = "refs/heads/main"
    }

    helm {
      chart_path   = "charts/web"
      release_name = "web"
      namespace    = "web"
      values_files = ["charts/web/values-prod.yaml"]

      upgrade {
        retries  = 2
        strategy = "rollback"
      }
    }

    config {
      drift_mode       = "enabled"
      ignore_paths     = ["/spec/replicas"]
      timeout          = "5m"
      kubernetes_prune = true
    }
  }
}
```

## Arguments Reference

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `name` | string | ✅ yes | Name of the workflow. |
| `on_destroy` | string | ❌ no | What happens to the deployed artifacts on destroy: `destroy` or `detach`. Defaults to `destroy`. |
| `engine` | string | ❌ no | `edgeStack` or `reconcile`. Left unset, Portainer uses `edgeStack`. `edgeStack` → `reconcile` migrates in place; `reconcile` → anything else forces a new resource. Requires Portainer BE 3.0 or later. |
| `reconcile_interval` | string | ❌ no | How often every artifact reconciles even when its source has not moved, for example `5m`. Required with `engine = "reconcile"`, rejected otherwise. Requires Portainer BE 3.0 or later. |
| `artifact` | block | ✅ yes | The artifacts the workflow deploys. Repeatable. |

### `artifact`

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `name` | string | ✅ yes | Name of the artifact. Portainer cannot rename an artifact, so changing it forces a new resource. |
| `type` | string | ❌ no | `stack` or `edgeStack`. Defaults to `edgeStack`. Ignored on the `reconcile` engine. |
| `deployment_type` | string | ✅ yes | Deployment type, for example `compose` or `kubernetes`. |
| `edge_group_ids` | list(number) | ❌ no | Edge groups the artifact is deployed to. Every artifact needs at least one of `edge_group_ids`, `environment_ids`, `environment_group_ids`; the `edgeStack` engine only deploys to Edge groups. |
| `environment_ids` | list(number) | ❌ no | Environments targeted directly. `reconcile` engine only. Requires Portainer BE 3.0 or later. |
| `environment_group_ids` | list(number) | ❌ no | Environment groups targeted, including environments added to them later. `reconcile` engine only. Requires Portainer BE 3.0 or later. |
| `dependencies` | list(string) | ❌ no | Names of other artifacts in this workflow that must be deployed before this one. Requires Portainer BE 3.0 or later. |
| `file` | block | ✅ yes | Files the artifact is built from. Repeatable. |
| `helm` | block | ❌ no | Deploys the artifact as a Helm chart. At most one. `reconcile` engine only. Requires Portainer BE 3.0 or later. |
| `config` | block | ❌ no | Deployment options. At most one. |

#### `artifact.file`

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `source_id` | number | ✅ yes | GitOps source the file comes from. |
| `path` | string | ✅ yes | Path within the source. |
| `ref` | string | ✅ yes | Git reference to read the file at. |

#### `artifact.helm`

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `chart_path` | string | ✅ yes | Directory of the chart within the artifact's sources. |
| `release_name` | string | ❌ no | Helm release name. Portainer generates one when left out. |
| `namespace` | string | ❌ no | Namespace the release is installed into. |
| `values_files` | list(string) | ❌ no | Values files merged over the chart's `values.yaml`, in order. |
| `install` | block | ❌ no | What happens when the first install fails: `retries`, `remediate_last_failure`, `strategy`. At most one. |
| `upgrade` | block | ❌ no | What happens when an upgrade fails: `retries`, `remediate_last_failure`, `strategy`. At most one. |

`install` and `upgrade` take:

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `retries` | number | ❌ no | Further attempts before the failure is remediated. |
| `remediate_last_failure` | bool | ❌ no | Whether the final failure is undone. Only sent when configured. |
| `strategy` | string | ❌ no | `rollback` or `uninstall`. Portainer defaults to `uninstall` for an install and `rollback` for an upgrade. |

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
| `kubernetes_prune` | bool | ❌ no | Delete resources a previous deployment applied that the manifests no longer declare. Only sent when configured; Portainer prunes by default. Requires Portainer BE 3.0 or later. |
| `kubernetes_force_apply` | bool | ❌ no | Delete and recreate a resource whose immutable fields changed. Only sent when configured. Requires Portainer BE 3.0 or later. |
| `drift_mode` | string | ❌ no | `disabled`, `warn` (report) or `enabled` (correct). `reconcile` engine only. Requires Portainer BE 3.0 or later. |
| `ignore_paths` | list(string) | ❌ no | JSON pointers whose differences are not drift, for example `/spec/replicas`. `reconcile` engine only. Requires Portainer BE 3.0 or later. |
| `timeout` | string | ❌ no | How long a reconcile waits for what it applied to become ready, for example `5m`. `reconcile` engine only. Requires Portainer BE 3.0 or later. |

## Attributes Reference

| Name | Type | Description |
|------|------|-------------|
| `workflow_id` | number | Identifier of the workflow. |
| `artifact.artifact_id` | number | Identifier Portainer assigned to the artifact, which the update call needs. On the `reconcile` engine it is the reconciled deployment's identifier, as `portainer_gitops_workflow_deployment_resources` takes it. |

## Lifecycle & Behavior

**`on_destroy` is a real choice.** Portainer has two removal endpoints that do different things: `destroy` tears the deployed stacks down along with the workflow, `detach` leaves them running and only unlinks them from GitOps. The default is `destroy`, which is what removing a resource normally means in Terraform.

**An artifact cannot be renamed.** Portainer's update payload has no name field at all, so renaming one in the configuration forces the workflow to be replaced. Without that, the new name would be accepted into state and never sent, leaving the configuration and Portainer quietly disagreeing.

**Only artifact identifiers are read back.** The update payload keys artifacts by the identifier Portainer assigned, so those are read into state. The rest of an artifact is driven by the configuration: Portainer normalises the file and config fields, and reading them back would make a stable configuration churn.

**Migrating to the reconcile engine is one-way.** Changing `engine` from `edgeStack` (or unset) to `reconcile` is an in-place update: Portainer removes the workflow's Edge stacks and the reconcile engine adopts what they deployed, without recreating anything. Portainer cannot move a workflow back, so changing `engine` away from `reconcile` replaces the workflow. `reconcile_interval` is required to create or migrate a reconcile workflow; removing it later leaves the recorded interval alone. Use `portainer_gitops_workflow_reconcile` to reconcile on demand.

**`engine` is derived, not read.** Portainer reports no engine of its own, so the provider derives it from the artifact types: a reconciled artifact is a `gitopsDeployment`. An unset `engine` stays unset in state for an Edge stack workflow, so a Portainer 2.45 workflow plans no change. When the reconcile engine was asked for but the server still reports Edge stacks - Portainer before 3.0 ignores the field - the apply fails and names the requirement.

**Portainer 2.45 compatibility.** None of the 3.0 attributes is sent unless configured, so a configuration that does not use them sends exactly what it did before. They are not read back either, like the rest of an artifact. `kubernetes_prune`, `kubernetes_force_apply` and `remediate_last_failure` are only sent when configured, so a configured `false` reaches Portainer while a removed one leaves Portainer's recorded choice alone.

## Import

```bash
terraform import portainer_gitops_workflow.platform 4
```

The import ID is the workflow identifier. An imported workflow has no artifact blocks in state, so they have to be supplied in the configuration before the first apply.
