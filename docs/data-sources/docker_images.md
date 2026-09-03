# Data Source Documentation: `portainer_docker_images`

# portainer_docker_images
Lists the images stored in a Docker environment, optionally marking which ones a container is using.

## Example Usage

```hcl
data "portainer_docker_images" "prod" {
  environment_id = 1
  with_usage     = true
}

output "reclaimable_bytes" {
  value = sum([for i in data.portainer_docker_images.prod.images : i.size if !i.used])
}
```

## Arguments Reference

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `environment_id` | number | ✅ yes | Environment (endpoint) identifier of the Docker environment. |
| `with_usage` | bool | ❌ no | Ask Portainer which images are in use. Costs an extra pass over the containers, so it defaults to false. |

## Attributes Reference

| Name | Type | Description |
|------|------|-------------|
| `images` | list | Images in the environment. Each entry has `id`, `tags`, `size`, `created`, `node_name` and `used`. |
| `total_size` | number | Combined size of every listed image, in bytes. |
| `unused_count` | number | Images no container uses. Only meaningful with `with_usage`. |

`used` is always false unless `with_usage` is set — do not read it as "nothing is running" when the flag is off. An image with no `tags` is dangling.
