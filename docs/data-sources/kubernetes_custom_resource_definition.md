# Data Source Documentation: `portainer_kubernetes_custom_resource_definition`

# portainer_kubernetes_custom_resource_definition

> **Business Edition only.** This endpoint does not exist in Portainer CE.


Inspects one custom resource definition.

## Example Usage

```hcl
data "portainer_kubernetes_custom_resource_definition" "widgets" {
  endpoint_id = portainer_environment.prod.id
  name        = "widgets.example.com"
}
```

## Arguments Reference

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `endpoint_id` | number | ✅ yes | Identifier of the Kubernetes environment to query. |
| `name` | string | ✅ yes | Name of the definition to inspect. |

## Attributes Reference

| Name | Type | Description |
|------|------|-------------|
| `group` | string | API group the definition belongs to. |
| `scope` | string | Whether its resources are `Namespaced` or `Cluster` scoped. |
| `creation_date` | string | When the definition was created. |
| `release_name` | string | Helm release that installed it. |
| `release_namespace` | string | Namespace of that Helm release. |
| `release_version` | string | Version of that Helm release. |

`scope` is what decides whether `portainer_kubernetes_custom_resource` needs a `namespace`.
