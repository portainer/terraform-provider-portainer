# Data Source Documentation: `portainer_kubernetes_custom_resources`

# portainer_kubernetes_custom_resources

> **Business Edition only.** This endpoint does not exist in Portainer CE.


Lists the custom resources of one definition.

## Example Usage

```hcl
data "portainer_kubernetes_custom_resources" "widgets" {
  endpoint_id = portainer_environment.prod.id
  definition  = "widgets.example.com"
}
```

## Arguments Reference

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `endpoint_id` | number | ✅ yes | Identifier of the Kubernetes environment to query. |
| `definition` | string | ✅ yes | Name of the definition whose resources are listed. |

## Attributes Reference

| Name | Type | Description |
|------|------|-------------|
| `resources` | list(object) | The custom resources of that definition. |

### `resources`

| Name | Type | Description |
|------|------|-------------|
| `name` | string | Name of the resource. |
| `namespace` | string | Namespace of the resource, empty for a cluster-scoped one. |
| `definition_name` | string | Definition the resource belongs to. |
| `uid` | string | Kubernetes UID of the resource. |
| `creation_date` | string | When the resource was created. |
