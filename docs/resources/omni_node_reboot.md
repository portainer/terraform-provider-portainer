# Resource Documentation: `portainer_omni_node_reboot`

# portainer_omni_node_reboot

> **Business Edition only.** This endpoint does not exist in Portainer CE.


Reboots a node of an Omni cluster.

## Example Usage

```hcl
resource "portainer_omni_node_reboot" "cp_1" {
  credential_id = portainer_cloud_credentials.omni.id
  cluster       = portainer_omni_cluster.edge_fleet.name
  node          = "cp-1"

  triggers = {
    after_upgrade = portainer_omni_cluster.edge_fleet.talos_version
  }
}
```

## Arguments Reference

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `credential_id` | number | ✅ yes | Portainer cloud credential holding the Omni endpoint and service account key. |
| `cluster` | string | ✅ yes | Cluster the node belongs to. |
| `node` | string | ✅ yes | Node to reboot. |
| `triggers` | map | ❌ no | Arbitrary values that force another reboot when they change. |

A reboot is a one-shot action with no state to read back, so without a `triggers` value the resource runs once and then stays put.
