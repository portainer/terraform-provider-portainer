# 🐳 **Resource Documentation: `portainer_docker_image`**

# portainer_docker_image
The `portainer_docker_image` resource allows you to pull Docker images on a specific Portainer environment (endpoint).
You can optionally provide registry authentication for private registries.

## Example Usage

### Pull public image from Docker Hub
```hcl
resource "portainer_docker_image" "nginx_test" {
  endpoint_id = 1
  image       = "nginx:alpine"
}
```

### Pull private image with registry authentication
```hcl
resource "portainer_docker_image" "private_image" {
  endpoint_id   = 1
  image         = "myregistry.example.com/myimage:latest"
  registry_auth = "username:password"
}
```

## Lifecycle & Behavior
Image will be pulled (downloaded) to the Docker host behind the specified Portainer endpoint.

Deleting the resource will remove the image from the host.

An image removed from the host outside Terraform is detected on refresh and pulled again by the next apply.

Updating the image tag or name will trigger a re-pull of the new image.
- To delete a docker image created via Terraform, simply run:
```hcl
terraform destroy
```

## Arguments Reference
| Name           | Type   | Required   | Description                                                                 |
|----------------|--------|------------|-----------------------------------------------------------------------------|
| `endpoint_id`  | int    | ✅ yes     | ID of the Portainer environment (endpoint)                                  |
| `image`        | string | ✅ yes     | Full image name including tag (e.g., `nginx:alpine`)                         |
| `registry_auth`| string | 🚫 optional| Registry credentials in format `username:password` (for private registries) |
| `registry_auth_wo` | string | 🚫 optional | Write-only `registry_auth` (supports ephemeral values; never stored in Terraform state; Terraform 1.11+). Conflicts with `registry_auth`; requires `registry_auth_wo_version`. |
| `registry_auth_wo_version` | number | 🚫 optional | Version of `registry_auth_wo`; increment it to send a new value. Changing it forces resource recreation. |

> 🔐 If registry_auth is not set, the provider sends an empty authentication object ({}), which works for public registries like Docker Hub.

## Timeouts

This resource supports the following timeout configuration options:

| Timeout  | Default   | Description                          |
|----------|-----------|--------------------------------------|
| `create` | 10 minutes | Time to wait for the image to be pulled |
| `delete` | 5 minutes  | Time to wait for the image to be deleted |

### Example: Custom Timeouts

```hcl
resource "portainer_docker_image" "large_image" {
  endpoint_id = 1
  image       = "myregistry.example.com/large-image:latest"

  timeouts {
    create = "20m"
    delete = "10m"
  }
}
```

## Attributes Reference

| Name | Description              |
|------|--------------------------|
| `id` | Unique identifier in the format `endpointId-image` |
