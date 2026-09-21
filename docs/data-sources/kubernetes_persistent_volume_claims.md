# Data Source Documentation: `portainer_kubernetes_persistent_volume_claims`

# portainer_kubernetes_persistent_volume_claims

Lists the persistent volume claims in a cluster, or in one namespace.

## Example Usage

```hcl
data "portainer_kubernetes_persistent_volume_claims" "pending" {
  endpoint_id = portainer_environment.prod.id
}

output "unbound_claims" {
  value = [
    for c in data.portainer_kubernetes_persistent_volume_claims.pending.claims : "${c.namespace}/${c.name}"
    if c.phase != "Bound"
  ]
}
```

## Arguments Reference

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `endpoint_id` | number | ✅ yes | Identifier of the Kubernetes environment to query. |
| `namespace` | string | ❌ no | Only list claims in this namespace. Leave unset to list every claim in the cluster. |

## Attributes Reference

| Name | Type | Description |
|------|------|-------------|
| `claims` | list(object) | The persistent volume claims. |

### `claims`

| Name | Type | Description |
|------|------|-------------|
| `access_modes` | list(string) | Access modes the claim asks for. |
| `creation_date` | string | When the claim was created. |
| `human_readable_access_modes` | list(string) | The same access modes, spelled out for display. |
| `id` | string | Kubernetes UID of the claim. |
| `labels` | map(string) | Labels on the claim. |
| `name` | string | Name of the claim. |
| `namespace` | string | Namespace of the claim. |
| `owning_applications` | list(string) | Applications using the claim. An empty list on a bound claim means nothing is mounting it. |
| `phase` | string | Phase of the claim, for example `Bound` or `Pending`. A claim stuck in `Pending` is the one worth looking at. |
| `storage` | number | Requested size in bytes. |
| `storage_class` | string | Storage class the claim asks for. |
| `storage_request` | string | Requested size as Kubernetes spells it, for example `10Gi`. |
| `volume_mode` | string | Volume mode of the claim, for example `Filesystem`. |
| `volume_name` | string | Persistent volume the claim is bound to, empty while it is unbound. |

Portainer has a separate path for a single namespace; leaving `namespace` unset lists the whole cluster.
