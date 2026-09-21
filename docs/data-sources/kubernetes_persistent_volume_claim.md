# Data Source Documentation: `portainer_kubernetes_persistent_volume_claim`

# portainer_kubernetes_persistent_volume_claim

Inspects one persistent volume claim.

## Example Usage

```hcl
data "portainer_kubernetes_persistent_volume_claim" "data" {
  endpoint_id = portainer_environment.prod.id
  namespace   = "apps"
  name        = "data"
}
```

## Arguments Reference

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `endpoint_id` | number | ✅ yes | Identifier of the Kubernetes environment to query. |
| `name` | string | ✅ yes | Name of the claim to inspect. |
| `namespace` | string | ✅ yes | Namespace of the claim. |

## Attributes Reference

| Name | Type | Description |
|------|------|-------------|
| `access_modes` | list(string) | Access modes the claim asks for. |
| `creation_date` | string | When the claim was created. |
| `human_readable_access_modes` | list(string) | The same access modes, spelled out for display. |
| `id` | string | Kubernetes UID of the claim. |
| `labels` | map(string) | Labels on the claim. |
| `owning_applications` | list(string) | Applications using the claim. |
| `phase` | string | Phase of the claim, for example `Bound` or `Pending`. |
| `storage` | number | Requested size in bytes. |
| `storage_class` | string | Storage class the claim asks for. |
| `storage_request` | string | Requested size as Kubernetes spells it, for example `10Gi`. |
| `volume_mode` | string | Volume mode of the claim, for example `Filesystem`. |
| `volume_name` | string | Persistent volume the claim is bound to, empty while it is unbound. |

An empty `owning_applications` on a `Bound` claim means the storage is allocated but nothing is mounting it.
