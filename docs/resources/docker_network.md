# 🌐 **Resource Documentation: `portainer_docker_network`**

# portainer_docker_network
The `portainer_docker_network` resource allows you to manage Docker networks on a specific environment (endpoint) in Portainer.
You can define bridge or overlay networks, including driver-specific options, labels, and IPAM settings.

## Example Usage

### Create a basic bridge network
```hcl
resource "portainer_docker_network" "bridge_network" {
  endpoint_id = 1
  name        = "tf-bridge"
  driver      = "bridge"
  internal    = false
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
resource "portainer_docker_network" "with_ipam" {
  endpoint_id = 1
  name        = "tf-ipam"
  driver      = "bridge"
  attachable  = true

  ipam = {
    driver = "default"
    config = [
      {
        subnet = "192.168.100.0/24"
        gateway = "192.168.100.1"
      }
    ]
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
| Name           | Type         | Required    | Description                                                                 |
|----------------|--------------|-------------|-----------------------------------------------------------------------------|
| `endpoint_id`  | int          | ✅ yes      | ID of the environment where the network will be created                     |
| `name`         | string       | ✅ yes      | Name of the Docker network                                                  |
| `driver`       | string       | ✅ yes      | Network driver (`bridge`, `overlay`, `macvlan`, etc.)                      |
| `internal`     | bool         | 🚫 optional | Whether the network is internal (default: `false`)                         |
| `attachable`   | bool         | 🚫 optional | Whether containers can be attached manually (default: `false`)             |
| `ingress`      | bool         | 🚫 optional | Whether it's an ingress network (default: `false`)                         |
| `config_only`  | bool         | 🚫 optional | If this network is only configuration (default: `false`)                   |
| `config_from`  | string       | 🚫 optional | Name of another config-only network to inherit from                        |
| `enable_ipv4`  | bool         | 🚫 optional | Enable IPv4 networking (default: `true`)                                   |
| `enable_ipv6`  | bool         | 🚫 optional | Enable IPv6 networking (default: `false`)                                  |
| `options`      | map(string)  | 🚫 optional | Driver-specific options                                                     |
| `labels`       | map(string)  | 🚫 optional | Labels to apply to the network                                              |
| `ipam`         | object       | 🚫 optional | IPAM configuration, see below                                               |

### IPAM Configuration

| Name      | Type           | Required    | Description                                                    |
|-----------|----------------|-------------|----------------------------------------------------------------|
| `driver`  | string         | 🚫 optional | IPAM driver (default: `default`)                               |
| `config`  | list(object)   | 🚫 optional | List of IPAM subnet configs (with `subnet`, `gateway`)         |
| `options` | map(string)    | 🚫 optional | IPAM driver-specific options                                   |

```hcl
ipam = {
  driver = "default"
  config = [
    {
      subnet  = "192.168.1.0/24"
      gateway = "192.168.1.1"
    }
  ]
}
```

## Attributes Reference

| Name | Description              |
|------|--------------------------|
| `id` | ID of the created Docker network (as returned by Portainer) |
