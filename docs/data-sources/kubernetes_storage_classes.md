# Data Source Documentation: `portainer_kubernetes_storage_classes`

# portainer_kubernetes_storage_classes

Lists the storage classes a Kubernetes cluster offers, which is where the `storage_class` a claim should ask for comes from.

## Example Usage

```hcl
data "portainer_kubernetes_storage_classes" "all" {
  endpoint_id = portainer_environment.prod.id
}

output "default_storage_class" {
  value = one([for c in data.portainer_kubernetes_storage_classes.all.storage_classes : c.name if c.is_default])
}
```

## Arguments Reference

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `endpoint_id` | number | ✅ yes | Identifier of the Kubernetes environment to query. |

## Attributes Reference

| Name | Type | Description |
|------|------|-------------|
| `storage_classes` | list(object) | The storage classes the cluster offers. |

### `storage_classes`

| Name | Type | Description |
|------|------|-------------|
| `allow_volume_expansion` | bool | Whether volumes of this class can be grown after they are created. |
| `annotations` | map(string) | Annotations on the storage class. |
| `creation_date` | string | When the storage class was created. |
| `is_default` | bool | Whether this is the cluster's default storage class. |
| `labels` | map(string) | Labels on the storage class. |
| `mount_options` | list(string) | Mount options applied to volumes of this class. |
| `name` | string | Name of the storage class. |
| `parameters` | map(string) | Provisioner-specific parameters of the class. |
| `provisioner` | string | Provisioner backing the storage class. |
| `reclaim_policy` | string | What happens to a volume when its claim is released, for example `Delete` or `Retain`. |

A cluster with no class marked `is_default` will leave a claim that names no class stuck in `Pending`.
