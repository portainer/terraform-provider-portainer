# Data Source Documentation: `portainer_kubernetes_storage_class`

# portainer_kubernetes_storage_class

Inspects one storage class.

## Example Usage

```hcl
data "portainer_kubernetes_storage_class" "fast" {
  endpoint_id = portainer_environment.prod.id
  name        = "fast-ssd"
}
```

## Arguments Reference

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `endpoint_id` | number | ✅ yes | Identifier of the Kubernetes environment to query. |
| `name` | string | ✅ yes | Name of the storage class to inspect. |

## Attributes Reference

| Name | Type | Description |
|------|------|-------------|
| `allow_volume_expansion` | bool | Whether volumes of this class can be grown after they are created. |
| `annotations` | map(string) | Annotations on the storage class. |
| `creation_date` | string | When the storage class was created. |
| `is_default` | bool | Whether this is the cluster's default storage class. |
| `labels` | map(string) | Labels on the storage class. |
| `mount_options` | list(string) | Mount options applied to volumes of this class. |
| `parameters` | map(string) | Provisioner-specific parameters of the class. |
| `provisioner_name` | string | Provisioner backing the storage class. |
| `reclaim_policy` | string | What happens to a volume when its claim is released, for example `Delete` or `Retain`. |

`allow_volume_expansion` is what decides whether an existing claim of this class can be resized later.
