# Data Source Documentation: `portainer_docker_container_gpus`

# portainer_docker_container_gpus
Reports the GPUs assigned to a single container.

## Example Usage

```hcl
data "portainer_docker_container_gpus" "trainer" {
  environment_id = 1
  container_id   = "b3f1a2c4d5e6"
}

output "trainer_gpus" {
  value = data.portainer_docker_container_gpus.trainer.gpus
}
```

## Arguments Reference

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `environment_id` | number | ✅ yes | Environment (endpoint) identifier of the Docker environment. |
| `container_id` | string | ✅ yes | Identifier of the container to inspect. |

## Attributes Reference

| Name | Type | Description |
|------|------|-------------|
| `gpus` | string | GPUs assigned to the container, empty when it has none. |
