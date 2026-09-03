# Resource Documentation: `portainer_kubernetes_deployment_rollback`

# portainer_kubernetes_deployment_rollback
The `portainer_kubernetes_deployment_rollback` resource rolls a Kubernetes deployment back to an earlier rollout revision through Portainer's native rollback API, the equivalent of `kubectl rollout undo`.

A rollback is a one-shot operation, so this resource behaves like an action: it runs on create, has nothing to read back, and removing it from the configuration only drops it from state — the rollback is not undone. Changing any argument replaces the resource, which rolls back again.

Requires Portainer **2.45.0** or newer.

## Example Usage

### Roll back to the previous revision

```hcl
resource "portainer_kubernetes_deployment_rollback" "web_previous" {
  environment_id  = 1
  namespace       = "default"
  deployment_name = "web"
  revision        = 0
}
```

### Roll back to a specific revision

Use the `portainer_kubernetes_replicasets` data source to discover which revisions exist:

```hcl
data "portainer_kubernetes_replicasets" "web" {
  environment_id = 1
  namespace      = "default"
  deployment     = "web"
}

resource "portainer_kubernetes_deployment_rollback" "web_pinned" {
  environment_id  = 1
  namespace       = "default"
  deployment_name = "web"
  revision        = 3
}
```

## Arguments Reference

| Name              | Type   | Required | Default | Description                                                                                                                             |
|-------------------|--------|----------|---------|-----------------------------------------------------------------------------------------------------------------------------------------|
| `environment_id`  | number | Yes      | –       | Identifier of the Portainer Kubernetes environment the deployment belongs to. Changing this replaces the resource.                       |
| `namespace`       | string | Yes      | –       | Namespace the deployment lives in. Changing this replaces the resource.                                                                  |
| `deployment_name` | string | Yes      | –       | Name of the deployment to roll back. Changing this replaces the resource.                                                                |
| `revision`        | number | No       | `0`     | Rollout revision to roll back to. `0` rolls back to the previous revision. Changing this replaces the resource.                          |

Portainer answers with `404` when the requested revision does not exist or the deployment has no rollout history at all.

## Attributes Reference

| Name                      | Description                                                                                       |
|---------------------------|---------------------------------------------------------------------------------------------------|
| `id`                      | Composite identifier in the form `<environment_id>/<namespace>/<deployment_name>/rollback/<timestamp>`. |
| `rolled_back_to_replicas` | Desired replica count of the deployment after the rollback.                                       |
