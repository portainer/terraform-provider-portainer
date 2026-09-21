# Data Source Documentation: `portainer_omni_machines`

# portainer_omni_machines

> **Business Edition only.** This endpoint does not exist in Portainer CE.


Lists the machines Omni knows about, allocated or not.

## Example Usage

```hcl
data "portainer_omni_machines" "all" {
  credential_id = portainer_cloud_credentials.omni.id
}

output "unallocated_machines" {
  value = [for m in data.portainer_omni_machines.all.machines : m.machine_name if m.cluster == ""]
}
```

## Arguments Reference

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `credential_id` | number | ✅ yes | Portainer cloud credential holding the Omni endpoint and service account key. |

## Attributes Reference

| Name | Type | Description |
|------|------|-------------|
| `machines` | list(object) | The machines Omni knows about. |

### `machines`

| Name | Type | Description |
|------|------|-------------|
| `machine_name` | string | Name of the machine. |
| `cluster` | string | Cluster the machine belongs to, empty while unallocated. |
| `connected` | bool | Whether the machine is connected to Omni. |
| `maintenance` | bool | Whether the machine is in maintenance mode. |
| `management_address` | string | Address Omni manages the machine on. |
| `power_state` | string | Power state Omni reports. |
| `role` | string | Role the machine plays in its cluster. |
| `talos_version` | string | Talos version running on the machine. |
| `last_error` | string | Most recent error Omni recorded, empty when there is none. |
| `labels` | map(string) | Labels Omni has on the machine. |

This is the summary view. For the block devices and network links of one machine - which is where `install_disk` and `interface` values for `portainer_omni_cluster` come from - use `portainer_omni_machine`.
