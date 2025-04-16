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

> 🔐 If registry_auth is not set, the provider sends an empty authentication object ({}), which works for public registries like Docker Hub.

## Attributes Reference

| Name | Description              |
|------|--------------------------|
| `id` | Unique identifier in the format `endpointId-image` |
