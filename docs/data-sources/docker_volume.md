# 📦 **Data Source Documentation: `portainer_docker_volume`**

# portainer_docker_volume
The `portainer_docker_volume` data source allows you to look up an existing Docker volume within a specific Portainer environment.

## Example Usage

### Look up a Docker volume by name

```hcl
data "portainer_docker_volume" "data" {
  endpoint_id = 1
  name        = "db-data"
}

output "volume_mount" {
  value = data.portainer_docker_volume.data.mount_point
}
```

## Arguments Reference

| Name          | Type    | Required | Description                              |
|---------------|---------|----------|------------------------------------------|
| `endpoint_id` | integer | ✅ yes   | ID of the environment.                  |
| `name`        | string  | ✅ yes   | Name of the Docker volume.              |

## Attributes Reference

| Name          | Type   | Description                    |
|---------------|--------|--------------------------------|
| `id`          | string | Name/ID of the Docker volume.  |
| `driver`      | string | Volume driver (local/etc).     |
| `mount_point` | string | Path to the volume on the host.|
