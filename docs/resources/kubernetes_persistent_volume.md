# Resource Documentation: `portainer_kubernetes_persistent_volume`

# portainer_kubernetes_persistent_volume
The `portainer_kubernetes_persistent_volume` resource manages the reclaim policy of an existing Kubernetes PersistentVolume, and deletes the volume when the resource is destroyed.

Portainer has **no API to create a PersistentVolume**, so this resource adopts a volume created elsewhere — by a manifest, by a StorageClass provisioner, or by a Helm chart. The reclaim policy is the one property Kubernetes lets you change on a bound volume in place, and it decides what happens to the underlying storage when its claim goes away.

Requires Portainer **2.45.0** or newer.

## Example Usage

```hcl
resource "portainer_kubernetes_persistent_volume" "data" {
  environment_id = 1
  name           = "pvc-8f3c1a2e-data"
  reclaim_policy = "Retain"
}

output "data_volume_claim" {
  value = portainer_kubernetes_persistent_volume.data.claim_ref
}
```

## Arguments Reference

| Name             | Type   | Required | Description                                                                                              |
|------------------|--------|----------|------------------------------------------------------------------------------------------------------------|
| `environment_id` | number | ✅ yes   | Identifier of the Portainer Kubernetes environment. Changing it replaces the resource.                       |
| `name`           | string | ✅ yes   | Name of an existing PersistentVolume. Changing it replaces the resource.                                     |
| `reclaim_policy` | string | ✅ yes   | Reclaim policy: `Retain`, `Delete` or `Recycle`.                                                              |

## Attributes Reference

| Name            | Type   | Description                                                                                |
|-----------------|--------|--------------------------------------------------------------------------------------------|
| `id`            | string | Composite identifier in the form `<environment_id>/<name>`.                                  |
| `phase`         | string | Lifecycle phase: `Available`, `Bound`, `Released`, `Failed` or `Pending`.                    |
| `capacity`      | string | Storage capacity as a Kubernetes quantity string (for example `10Gi`).                        |
| `storage_class` | string | StorageClass the volume belongs to, empty for a statically provisioned volume.                |
| `access_modes`  | list   | Access modes the volume supports, such as `ReadWriteOnce`.                                    |
| `claim_ref`     | string | Claim bound to the volume as `<namespace>/<name>`, empty while unbound.                       |

Destroying this resource **deletes the PersistentVolume**. With `reclaim_policy = "Retain"` the underlying storage survives that deletion; with `Delete` it does not.
