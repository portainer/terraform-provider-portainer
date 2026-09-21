# Data Source Documentation: `portainer_kubernetes_volume`

# portainer_kubernetes_volume

Inspects one volume, joining its persistent volume, its claim and its storage class.

## Example Usage

```hcl
data "portainer_kubernetes_volume" "data" {
  endpoint_id = portainer_environment.prod.id
  namespace   = "apps"
  volume      = "data"
}

output "volume_reclaim_policy" {
  value = data.portainer_kubernetes_volume.data.persistent_volume_reclaim_policy
}
```

## Arguments Reference

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `endpoint_id` | number | ✅ yes | Identifier of the Kubernetes environment to query. |
| `namespace` | string | ✅ yes | Namespace of the volume. |
| `volume` | string | ✅ yes | Name of the volume to inspect. |

## Attributes Reference

| Name | Type | Description |
|------|------|-------------|
| `claim_name` | string | Name of the claim bound to the volume. |
| `claim_phase` | string | Phase of the claim, for example `Bound` or `Pending`. |
| `claim_storage_request` | string | Size the claim asks for, as Kubernetes spells it. |
| `details` | string | The whole response as Portainer returns it, encoded as JSON. Decode it with `jsondecode()` for anything the attributes above do not cover. |
| `persistent_volume_name` | string | Name of the persistent volume behind the claim. |
| `persistent_volume_reclaim_policy` | string | What happens to the volume when its claim is released. |
| `persistent_volume_status` | string | Phase of the persistent volume, for example `Bound`. |
| `storage_class_name` | string | Storage class backing the volume. |
| `storage_class_provisioner` | string | Provisioner of that storage class. |

The CSI and object-reference structures in the full response are too deep and too Kubernetes-version-specific to flatten usefully, so `details` carries the whole response as JSON alongside the fields above.
