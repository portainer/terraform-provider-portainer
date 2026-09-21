# Resource Documentation: `portainer_omni_cluster`

# portainer_omni_cluster

> **Business Edition only.** This endpoint does not exist in Portainer CE.


Provisions a Talos cluster through Siderolabs Omni. Creating one also creates the Portainer environment that fronts it, whose identifier is reported back as `endpoint_id`.

## Example Usage

```hcl
data "portainer_omni_talos_versions" "supported" {
  credential_id = portainer_cloud_credentials.omni.id
}

resource "portainer_omni_cluster" "edge_fleet" {
  credential_id      = portainer_cloud_credentials.omni.id
  name               = "edge-fleet"
  talos_version      = data.portainer_omni_talos_versions.supported.talos_versions[0]
  kubernetes_version = "v1.31.0"

  labels = {
    environment = "production"
  }

  control_plane {
    machine {
      name             = "cp-1"
      hostname         = "cp-1"
      install_disk     = "/dev/sda"
      nameservers      = ["1.1.1.1"]
      system_disk_size = 50

      interface {
        interface = "eth0"
        addresses = ["10.0.0.11/24"]

        route {
          network = "0.0.0.0/0"
          gateway = "10.0.0.1"
        }
      }
    }
  }

  worker {
    name = "pool-a"

    machine {
      name         = "worker-1"
      install_disk = "/dev/sda"

      interface {
        interface = "eth0"
        dhcp      = true
      }

      user_disk {
        volume_name = "data"
        size        = 0 # take whatever space is left
      }
    }
  }
}
```

## Arguments Reference

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `credential_id` | number | ✅ yes | Portainer cloud credential holding the Omni endpoint and service account key. Changing it forces a new resource. |
| `name` | string | ✅ yes | Name of the cluster. Omni cannot rename a cluster, so changing it forces a new resource. |
| `kind` | string | ❌ no | Omni resource kind of the cluster. |
| `kubernetes_version` | string | ❌ no | Kubernetes version to run. Leave unset to let Omni pick and read the result back. |
| `talos_version` | string | ❌ no | Talos version to run. |
| `labels` | map(string) | ❌ no | Labels to set on the Omni cluster resource. |
| `cluster_config` | string | ❌ no | Raw Omni cluster template, as an alternative to the blocks below. Only read at creation, so changing it forces a new resource. |
| `portainer_url` | string | ❌ no | URL the provisioned nodes reach Portainer on. |
| `tunnel_server_address` | string | ❌ no | Address the provisioned nodes reach Portainer's tunnel server on. |
| `validate_before_create` | bool | ❌ no | Validate the configuration with Portainer before provisioning. Defaults to `true`. |
| `cluster_patch` | block | ❌ no | Talos configuration patches applied to the cluster as a whole. Repeatable. |
| `control_plane` | block | ❌ no | The cluster's control plane. At most one. |
| `worker` | block | ❌ no | The cluster's worker pool. At most one. |

### `cluster_patch`

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `id_override` | string | ❌ no | Identifier Omni stores the patch under. |
| `annotation_name` | string | ❌ no | Name annotation Omni shows for the patch. |
| `inline` | string | ❌ no | Patch body as a JSON object. Use `jsonencode({...})`. |

### `control_plane` and `worker`

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `kind` | string | ❌ no | Omni resource kind of the group. |
| `name` | string | ❌ no | Name of the worker pool. `worker` only. |
| `machine` | block | ❌ no | Machines in the group. Repeatable. |

### `machine`

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `name` | string | ✅ yes | Name of the machine as Omni knows it. |
| `hostname` | string | ❌ no | Hostname to configure on the machine. |
| `kind` | string | ❌ no | Omni resource kind of the machine. |
| `install_disk` | string | ❌ no | Block device Talos is installed onto. |
| `nameservers` | list(string) | ❌ no | DNS servers to configure. |
| `system_disk_size` | number | ❌ no | Size of the ephemeral system volume in GiB. |
| `user_disk` | block | ❌ no | A user volume carved out of the remaining space. At most one. |
| `interface` | block | ❌ no | Network interfaces to configure. Repeatable. |
| `patch` | block | ❌ no | Talos machine configuration patches. Repeatable. |

#### `machine.user_disk`

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `volume_name` | string | ✅ yes | Name of the volume. |
| `size` | number | ❌ no | Size in GiB. Zero means take all the space that is left, and is sent as such. |

#### `machine.interface`

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `interface` | string | ✅ yes | Name of the interface, for example `eth0`. |
| `dhcp` | bool | ❌ no | Whether the interface takes its address from DHCP. |
| `addresses` | list(string) | ❌ no | Static addresses in CIDR notation. |
| `route` | block | ❌ no | Static routes through the interface. Repeatable. |

#### `machine.interface.route`

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `network` | string | ✅ yes | Destination network in CIDR notation. `0.0.0.0/0` for a default route. |
| `gateway` | string | ✅ yes | Gateway the route goes through. |

#### `machine.patch`

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `id_override` | string | ❌ no | Identifier Omni stores the patch under. |
| `inline` | string | ❌ no | Patch body as a JSON object. |

## Attributes Reference

| Name | Type | Description |
|------|------|-------------|
| `endpoint_id` | number | Portainer environment created for the cluster. |
| `phase` | number | Provisioning phase Omni reports. |
| `ready` | bool | Whether Omni reports the cluster as ready. |
| `available` | bool | Whether Omni reports the cluster as available. |
| `control_plane_ready` | bool | Whether the control plane has come up. |
| `kubernetes_api_ready` | bool | Whether the Kubernetes API is answering. |
| `machines_total` | number | Machines Omni counts in the cluster. |
| `machines_requested` | number | Machines the cluster asked for. |
| `machines_connected` | number | Machines currently connected to Omni. |
| `machines_healthy` | number | Machines Omni considers healthy. |
| `disk_encryption` | bool | Whether disk encryption is enabled. |
| `workload_proxy_enabled` | bool | Whether Omni's workload proxy is enabled. |
| `embedded_discovery_service` | bool | Whether the embedded discovery service is used. |
| `backup_enabled` | bool | Whether Omni backs the cluster's etcd up on a schedule. |
| `backup_interval` | string | Interval between the scheduled etcd backups. |

## Lifecycle & Behavior

**Validation runs first by default.** Provisioning a Talos cluster touches real machines and takes a long time, so the configuration is sent to Portainer's validation endpoint before anything is created. A rejected configuration fails the apply without provisioning. Set `validate_before_create = false` to skip the check.

**The machine blocks are never read back.** Portainer's read reports the cluster's spec and status, not the machines it was built from. Terraform therefore cannot detect drift in the machine list: a node removed outside Terraform will not show up in a plan. What the read does populate is the version, feature, backup and status attributes above.

**Updates are sent as a difference.** Portainer's update endpoint is imperative - it takes machines to add and machine names to remove. The provider works that difference out from the change to the `machine` blocks, so adding one machine sends one machine rather than the whole list.

**Use the machine data sources to fill the blocks in.** `portainer_omni_machines` lists what Omni has available, and `portainer_omni_machine` reports the block devices and network links a machine actually has, which is where `install_disk` and `interface` values come from.

**Cancelling a provision in flight is not exposed.** Portainer has an endpoint for it, but it is an interactive escape hatch rather than declarative state: Terraform's answer to a provision that will not finish is to destroy the resource, which is the delete endpoint.

## Import

```bash
terraform import portainer_omni_cluster.edge_fleet 3/edge-fleet
```

The import ID is `<credential id>/<cluster name>`. An imported cluster has no machine blocks in state, so they have to be supplied in the configuration - and because they are never read back, the first plan after an import will not show them as a change.
