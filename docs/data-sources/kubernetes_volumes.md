# Data Source Documentation: `portainer_kubernetes_volumes`

# portainer_kubernetes_volumes

Lists the volumes in a cluster, or in one namespace.

## Example Usage

```hcl
data "portainer_kubernetes_volumes" "all" {
  endpoint_id       = portainer_environment.prod.id
  with_applications = true
}

output "volume_names" {
  value = [for v in jsondecode(data.portainer_kubernetes_volumes.all.volumes) : v.persistentVolumeClaim.name]
}
```

## Arguments Reference

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `endpoint_id` | number | ✅ yes | Identifier of the Kubernetes environment to query. |
| `namespace` | string | ❌ no | Only list volumes in this namespace. Leave unset to list every volume in the cluster. |
| `with_applications` | bool | ❌ no | Whether to include the applications using each volume, which costs Portainer an extra lookup. |

## Attributes Reference

| Name | Type | Description |
|------|------|-------------|
| `volumes` | string | The volumes as Portainer returns them, encoded as JSON. Decode it with `jsondecode()`. Use `portainer_kubernetes_volume` for a typed view of one volume. |

Portainer does not give this listing a fixed shape in its own specification, so it is carried through as JSON rather than flattened into attributes that could go stale. For a typed view of a single volume, use `portainer_kubernetes_volume`.
