# 🌐 **Resource Documentation: `portainer_docker_network`**

# portainer_docker_network
The `portainer_docker_network` resource allows you to manage Docker networks on a specific environment (endpoint) in Portainer.
You can define bridge or overlay networks, including driver-specific options, labels, and IPAM settings.

## Example Usage

### Create a basic bridge network
```hcl
resource "portainer_docker_network" "bridge_network" {
  endpoint_id = 1
  name        = "new-bridge-network"
}
```

### Create an attachable bridge network with options and labels
```hcl
resource "portainer_docker_network" "bridge_network_with_configuration" {
  endpoint_id = 1
  name        = "new-bridge-network-with-configuration"
  attachable  = true

  options = {
    "com.docker.network.bridge.enable_icc"           = "true"
    "com.docker.network.bridge.enable_ip_masquerade" = "true"
  }

  labels = {
    "env"     = "test"
    "project" = "terraform"
  }
}
```

### Create a network with custom IPAM config
```hcl
resource "portainer_docker_network" "network_with_ipam" {
  endpoint_id = 1
  name        = "new-ipam-network"
  driver      = "bridge"
  attachable  = true
  ipam_driver = "default"

  ipam_config {
    subnet    = "192.168.100.0/24"
    gateway   = "192.168.100.1"
    ip_range  = "192.168.100.0/25"
  }

  ipam_options = {
    foo = "bar"
  }
}
```

## Lifecycle & Behavior
- To delete a docker netowrk created via Terraform, simply run:
```hcl
terraform destroy
```
Docker networks are immutable in Portainer. To update, you must destroy and recreate them.
- To modify a name (e.g., make it dynamic), update the attributes and re-apply:
```hcl
terraform apply
```

## Arguments Reference

| Name            | Type        | Required    | Description                                                    |
|-----------------|-------------|-------------|----------------------------------------------------------------|
| `endpoint_id`   | int         | ✅ yes       | ID of the environment where the network will be created       |
| `name`          | string      | ✅ yes       | Name of the Docker network                                    |
| `driver`        | string      | 🚫 optional | Network driver (`bridge`, `overlay`, `macvlan`, etc.), default: `bridge` |
| `scope`         | string      | 🚫 optional | Network scope (`local`, `swarm`), default: `local`             |
| `internal`      | bool        | 🚫 optional | Whether the network is internal (default: `false`)             |
| `attachable`    | bool        | 🚫 optional | Whether containers can be attached manually (default: `false`) |
| `ingress`       | bool        | 🚫 optional | Whether it's an ingress network (default: `false`)             |
| `config_only`   | bool        | 🚫 optional | If this network is only configuration (default: `false`)       |
| `config_from`   | string      | 🚫 optional | Name of another config-only network to inherit from            |
| `enable_ipv4`   | bool        | 🚫 optional | Enable IPv4 networking (default: `true`)                       |
| `enable_ipv6`   | bool        | 🚫 optional | Enable IPv6 networking (default: `false`)                      |
| `options`       | map(string) | 🚫 optional | Driver-specific options                                        |
| `labels`        | map(string) | 🚫 optional | Labels to apply to the network                                 |
| `swarm_node_id` | string      | 🚫 optional | Name of the swarm node to target when creating config-only networks |
| `ipam_driver`   | string      | 🚫 optional | IPAM driver name (default: `default`)                          |
| `ipam_options`  | map(string) | 🚫 optional | IPAM driver-specific options                                   |
| `ipam_config`   | block       | 🚫 optional | IPAM configuration block (can be repeated)                     |

### `ipam_config` Block

| Name                  | Type        | Required | Description                                       |
|-----------------------|-------------|----------|---------------------------------------------------|
| `subnet`              | string      | ✅ yes    | The subnet in CIDR format (e.g. `192.168.0.0/24`) |
| `ip_range`            | string      | 🚫 no     | Optional IP range within the subnet               |
| `gateway`             | string      | 🚫 no     | Gateway IP for the subnet                         |
| `auxiliary_addresses` | map(string) | 🚫 no     | Optional key-value pairs of reserved addresses    |

## Attributes Reference

| Name | Description                                                   |
|------|---------------------------------------------------------------|
| `id` | ID of the created Docker network (as returned by Portainer)   |
| `resource_control_id` | ID of the automatically generated Portainer ResourceControl for this network |

## Import

Docker networks can be imported using a composite ID in the form `<endpoint_id>:<network_id>`, where `<endpoint_id>` is the numeric ID of the Portainer environment and `<network_id>` is the Docker network ID (a string hash, not a number):

```shell
terraform import portainer_docker_network.example 1:a1b2c3d4e5f6
```
