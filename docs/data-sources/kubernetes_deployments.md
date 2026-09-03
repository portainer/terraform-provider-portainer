# Data Source Documentation: `portainer_kubernetes_deployments`

# portainer_kubernetes_deployments
The `portainer_kubernetes_deployments` data source lists Kubernetes deployments from a Portainer-managed environment through Portainer's native deployment API, either across the whole cluster or within a single namespace.

Requires Portainer **2.45.0** or newer.

## Example Usage

```hcl
data "portainer_kubernetes_deployments" "all" {
  environment_id = 1
}

output "deployment_names" {
  value = [for d in data.portainer_kubernetes_deployments.all.deployments : "${d.namespace}/${d.name}"]
}
```

### Detect rollouts that have not finished

```hcl
output "rollouts_in_progress" {
  value = [
    for d in data.portainer_kubernetes_deployments.all.deployments : "${d.namespace}/${d.name}"
    if d.observed_generation < d.generation || d.ready_replicas < d.replicas
  ]
}
```

## Arguments Reference

| Name             | Type   | Required | Description                                                                                                            |
|------------------|--------|----------|------------------------------------------------------------------------------------------------------------------------|
| `environment_id` | number | Yes      | Environment (endpoint) identifier of the Kubernetes environment to query.                                               |
| `namespace`      | string | No       | Namespace to list deployments from. Leave unset to list deployments across every namespace the API token can access.    |
| `label_selector` | string | No       | Kubernetes label selector used to filter the deployments (for example `app=nginx`).                                     |
| `field_selector` | string | No       | Kubernetes field selector used to filter the deployments (for example `metadata.name=web`).                              |

## Attributes Reference

| Name          | Type | Description                                                                                                                                                                                                                      |
|---------------|------|----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `deployments` | list | Deployments matching the query. Each entry has `name`, `namespace`, `labels`, `images`, `replicas`, `ready_replicas`, `available_replicas`, `updated_replicas`, `unavailable_replicas`, `generation`, `observed_generation` and `creation_timestamp`. |

`replicas` is the desired count from the deployment spec; the Kubernetes default of `1` is reported when the spec leaves it unset. `observed_generation` below `generation` means the deployment controller has not yet acted on the latest spec change.
