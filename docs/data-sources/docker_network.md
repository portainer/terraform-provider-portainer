# 🌐 **Data Source Documentation: `portainer_docker_network`**

# portainer_docker_network
The `portainer_docker_network` data source allows you to look up an existing Docker network within a specific Portainer environment.

## Example Usage

### Look up a Docker network by name

```hcl
data "portainer_docker_network" "my_network" {
  endpoint_id = 1
  name        = "frontend-network"
}

output "network_id" {
  value = data.portainer_docker_network.my_network.id
}
```

## Arguments Reference

| Name          | Type    | Required | Description                              |
|---------------|---------|----------|------------------------------------------|
| `endpoint_id` | integer | ✅ yes   | ID of the environment.                  |
| `name`        | string  | ✅ yes   | Name of the Docker network.             |

## Attributes Reference

| Name     | Type   | Description                    |
|----------|--------|--------------------------------|
| `id`     | string | ID of the Docker network.      |
| `driver` | string | Network driver (bridge/overlay).|
| `scope`  | string | Network scope (local/swarm).   |
