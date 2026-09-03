# Data Source Documentation: `portainer_kubernetes_persistent_volumes`

# portainer_kubernetes_persistent_volumes
Lists the PersistentVolumes of a cluster, including which claim each is bound to and which ones are left over.

## Example Usage

```hcl
data "portainer_kubernetes_persistent_volumes" "prod" {
  environment_id = 1
}

# Storage whose claim is gone but which Retain kept from being reclaimed.
output "orphaned_storage" {
  value = [
    for v in data.portainer_kubernetes_persistent_volumes.prod.persistent_volumes : v.name
    if v.phase == "Released"
  ]
}
```

## Arguments Reference

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `environment_id` | number | ✅ yes | Environment (endpoint) identifier of the Kubernetes environment. |

## Attributes Reference

| Name | Type | Description |
|------|------|-------------|
| `persistent_volumes` | list | Volumes in the cluster. Each entry has `name`, `phase`, `capacity`, `storage_class`, `reclaim_policy`, `access_modes` and `claim_ref`. |
| `released_count` | number | Number of volumes in the `Released` phase. |

Pair this with `portainer_kubernetes_persistent_volume` to pin the reclaim policy of the volumes that matter.
