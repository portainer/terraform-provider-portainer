# Data Source Documentation: `portainer_omni_machine`

# portainer_omni_machine

> **Business Edition only.** This endpoint does not exist in Portainer CE.


Inspects one Omni machine, including the hardware and network detail needed to write a `portainer_omni_cluster` machine block.

## Example Usage

```hcl
data "portainer_omni_machine" "cp_1" {
  credential_id = portainer_cloud_credentials.omni.id
  machine_name  = "cp-1"
}

output "system_disk" {
  value = one([for d in data.portainer_omni_machine.cp_1.block_devices : d.linux_name if d.system_disk])
}
```

## Arguments Reference

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `credential_id` | number | ✅ yes | Portainer cloud credential holding the Omni endpoint and service account key. |
| `machine_name` | string | ✅ yes | Name of the machine to inspect. |

## Attributes Reference

| Name | Type | Description |
|------|------|-------------|
| `cluster` | string | Cluster the machine belongs to, empty while unallocated. |
| `connected` | bool | Whether the machine is connected to Omni. |
| `maintenance` | bool | Whether the machine is in maintenance mode. |
| `management_address` | string | Address Omni manages the machine on. |
| `power_state` | string | Power state Omni reports. |
| `role` | string | Role the machine plays in its cluster. |
| `talos_version` | string | Talos version running on the machine. |
| `last_error` | string | Most recent error Omni recorded. |
| `labels` | map(string) | Labels Omni has on the machine. |
| `architecture` | string | CPU architecture. |
| `hostname` | string | Hostname the machine reports. |
| `domain_name` | string | Domain name the machine reports. |
| `addresses` | list(string) | Addresses configured on the machine. |
| `default_gateways` | list(string) | Default gateways configured on the machine. |
| `platform` | string | Platform from the machine's metadata. |
| `instance_id` | string | Instance identifier from the platform metadata. |
| `instance_type` | string | Instance type from the platform metadata. |
| `region` | string | Region from the platform metadata. |
| `zone` | string | Zone from the platform metadata. |
| `block_devices` | list(object) | Block devices Omni sees, with `linux_name`, `model`, `size`, `type` and `system_disk`. |
| `memory_modules` | list(object) | Memory modules, with `description` and `size_mb`. |
| `processors` | list(object) | Processors, with `description`, `manufacturer`, `core_count`, `thread_count` and `frequency`. |
| `network_links` | list(object) | Network links, with `linux_name`, `hardware_address`, `link_up` and `speed_mbps`. |
