# Resource Documentation: `portainer_kubernetes_deployment_scale`

# portainer_kubernetes_deployment_scale
The `portainer_kubernetes_deployment_scale` resource manages the replica count of a Kubernetes deployment that already exists in the cluster, using Portainer's native scale API.

It manages **only** the replica count: the deployment itself must be created elsewhere (for example with `portainer_kubernetes_application`, a stack, or Helm). The replica count is read back on refresh, so scaling done outside Terraform shows up as drift.

Deleting the resource stops managing the replica count and leaves the deployment running at its current scale — scaling has no inverse. Set `replicas = 0` to stop the workload.

Requires Portainer **2.45.0** or newer.

## Example Usage

```hcl
resource "portainer_kubernetes_deployment_scale" "web" {
  environment_id  = 1
  namespace       = "default"
  deployment_name = "web"
  replicas        = 3
}

output "web_ready_replicas" {
  value = portainer_kubernetes_deployment_scale.web.ready_replicas
}
```

## Arguments Reference

| Name              | Type   | Required | Description                                                                                                       |
|-------------------|--------|----------|-------------------------------------------------------------------------------------------------------------------|
| `environment_id`  | number | Yes      | Identifier of the Portainer Kubernetes environment the deployment belongs to. Changing this replaces the resource. |
| `namespace`       | string | Yes      | Namespace the deployment lives in. Changing this replaces the resource.                                            |
| `deployment_name` | string | Yes      | Name of the deployment to scale. Changing this replaces the resource.                                              |
| `replicas`        | number | Yes      | Desired number of replicas. Changing this scales the deployment in place.                                           |

## Attributes Reference

| Name                 | Description                                                                                            |
|----------------------|--------------------------------------------------------------------------------------------------------|
| `id`                 | Composite identifier in the form `<environment_id>/<namespace>/<deployment_name>/scale`.               |
| `ready_replicas`     | Number of replicas passing their readiness checks.                                                     |
| `available_replicas` | Number of replicas available for at least the configured minimum ready seconds.                        |

Both attributes are a snapshot taken at the moment of the request: the cluster needs time to converge, so a freshly scaled deployment normally reports fewer ready replicas than requested until the next refresh.
