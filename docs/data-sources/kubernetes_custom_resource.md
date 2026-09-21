# Data Source Documentation: `portainer_kubernetes_custom_resource`

# portainer_kubernetes_custom_resource

> **Business Edition only.** This endpoint does not exist in Portainer CE.


Fetches one custom resource.

## Example Usage

```hcl
data "portainer_kubernetes_custom_resource" "prod_widget" {
  endpoint_id = portainer_environment.prod.id
  definition  = "widgets.example.com"
  namespace   = "apps"
  name        = "prod"
}

output "widget_spec" {
  value = jsondecode(data.portainer_kubernetes_custom_resource.prod_widget.manifest).spec
}
```

## Arguments Reference

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `endpoint_id` | number | ✅ yes | Identifier of the Kubernetes environment to query. |
| `definition` | string | ✅ yes | Definition the resource belongs to. |
| `name` | string | ✅ yes | Name of the resource. |
| `namespace` | string | ❌ no | Namespace of the resource. Leave unset for a `Cluster` scoped definition. |

## Attributes Reference

| Name | Type | Description |
|------|------|-------------|
| `manifest` | string | The resource as Portainer returns it, encoded as JSON. |

A custom resource has no fixed shape, so it is carried through as JSON rather than flattened into attributes that could never cover every definition. Decode it with `jsondecode()`.

Portainer has separate paths for namespaced and cluster-scoped resources; leaving `namespace` unset is what selects the cluster-scoped one, which mirrors the definition's own `scope`.
