# 🖼️ **Data Source Documentation: `portainer_docker_image`**

# portainer_docker_image
The `portainer_docker_image` data source allows you to look up an existing Docker image within a specific Portainer environment.

## Example Usage

### Look up a Docker image by name

```hcl
data "portainer_docker_image" "nginx" {
  endpoint_id = 1
  name        = "nginx:latest"
}

output "image_id" {
  value = data.portainer_docker_image.nginx.id
}
```

## Arguments Reference

| Name          | Type    | Required | Description                               |
|---------------|---------|----------|-------------------------------------------|
| `endpoint_id` | integer | ✅ yes   | ID of the environment.                   |
| `name`        | string  | ✅ yes   | Full name of the image (e.g. `repo/img:tag`). |

## Attributes Reference

| Name | Type   | Description                  |
|------|--------|------------------------------|
| `id` | string | ID (Digest) of the Docker image. |
