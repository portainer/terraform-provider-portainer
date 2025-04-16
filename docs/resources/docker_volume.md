# 🧩 **Resource Documentation: `portainer_docker_volume`**

# portainer_docker_volume
The `portainer_docker_volume` resource allows you to create and manage Docker volumes via the Portainer API.

## Example Usage

### Create a Docker Volume
```hcl
resource "portainer_docker_image" "nginx_test" {
  endpoint_id = 1
  image       = "nginx:alpine"
}
```

### Pull private image with registry authentication
```hcl
resource "portainer_docker_volume" "example" {
  endpoint_id = 1
  name        = "my-test-volume"
  driver      = "local"

  driver_opts = {
    device = "tmpfs"
    o      = "size=100m,uid=1000"
    type   = "tmpfs"
  }

  labels = {
    env     = "test"
    managed = "terraform"
  }
}
```

## Lifecycle & Behavior
Creating a volume sends a POST request to the Docker API via Portainer.

Deleting a volume removes it via the corresponding DELETE call.
- You can recreate the volume by changing its name or any ForceNew parameter and running:
```hcl
terraform apply
```

- To destroy the volume:
```hcl
terraform destroy
```

## Arguments Reference
| Name         | Type         | Required | Description                                                       |
|--------------|--------------|----------|-------------------------------------------------------------------|
| `endpoint_id`| int          | ✅ yes   | ID of the Portainer environment (endpoint)                        |
| `name`       | string       | ✅ yes   | Name of the Docker volume                                         |
| `driver`     | string       | ✅ yes   | Volume driver to use (e.g., `local`, `custom`)                    |
| `driver_opts`| map(string)  | 🚫 optional | Driver-specific options (e.g., `device`, `type`, `o`)           |
| `labels`     | map(string)  | 🚫 optional | Key-value metadata to apply to the volume                        |

## Attributes Reference

| Name | Description              |
|------|--------------------------|
| `id` | Unique identifier of the volume |
