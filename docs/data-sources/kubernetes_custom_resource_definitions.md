# Data Source Documentation: `portainer_kubernetes_custom_resource_definitions`

# portainer_kubernetes_custom_resource_definitions

> **Business Edition only.** This endpoint does not exist in Portainer CE.


Lists the custom resource definitions installed in a Kubernetes cluster.

## Example Usage

```hcl
data "portainer_kubernetes_custom_resource_definitions" "all" {
  endpoint_id = portainer_environment.prod.id
}

output "crd_names" {
  value = [for c in data.portainer_kubernetes_custom_resource_definitions.all.definitions : c.name]
}
```

## Arguments Reference

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `endpoint_id` | number | ✅ yes | Identifier of the Kubernetes environment to query. |

## Attributes Reference

| Name | Type | Description |
|------|------|-------------|
| `definitions` | list(object) | The definitions installed in the cluster. |

### `definitions`

| Name | Type | Description |
|------|------|-------------|
| `name` | string | Name of the definition, which is what `portainer_kubernetes_custom_resources` takes as its `definition`. |
| `group` | string | API group the definition belongs to. |
| `scope` | string | Whether its resources are `Namespaced` or `Cluster` scoped. |
| `creation_date` | string | When the definition was created. |
| `release_name` | string | Helm release that installed it, empty when it was not installed by one. |
| `release_namespace` | string | Namespace of that Helm release. |
| `release_version` | string | Version of that Helm release. |

Custom resources are read-only in this provider. Portainer exposes list, inspect and delete for them but no create or update, because they come from applying manifests - use `portainer_kubernetes_manifest` for that.
