# Data Source Documentation: `portainer_omni_upgrade_status`

# portainer_omni_upgrade_status

> **Business Edition only.** This endpoint does not exist in Portainer CE.


Reports how a Kubernetes or Talos upgrade of an Omni cluster is going.

## Example Usage

```hcl
data "portainer_omni_upgrade_status" "talos" {
  credential_id = portainer_cloud_credentials.omni.id
  cluster       = portainer_omni_cluster.edge_fleet.name
  component     = "talos"
}

output "talos_upgrade_step" {
  value = data.portainer_omni_upgrade_status.talos.step
}
```

## Arguments Reference

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `credential_id` | number | ✅ yes | Portainer cloud credential holding the Omni endpoint and service account key. |
| `cluster` | string | ✅ yes | Cluster to report on. |
| `component` | string | ✅ yes | Which upgrade to report on: `kubernetes` or `talos`. |

## Attributes Reference

| Name | Type | Description |
|------|------|-------------|
| `status` | string | Status Omni reports for the upgrade. |
| `step` | string | Step the upgrade is currently on. |
| `phase` | number | Phase Omni reports for the upgrade. |
| `error` | string | Why the upgrade failed, empty when it has not. |
| `current_version` | string | Version the upgrade is moving to. |
| `last_version` | string | Version the cluster was on before the upgrade. |
| `available_versions` | list(string) | Versions the cluster can be upgraded to. |

Portainer tracks the Kubernetes and Talos upgrades separately and they can be at different steps, which is why `component` selects between them rather than both being reported at once.
