# ⚙️ **Data Source Documentation: `portainer_docker_config`**

# portainer_docker_config
The `portainer_docker_config` data source allows you to look up an existing Docker config within a specific Portainer Swarm environment.

## Example Usage

### Look up a Docker config by name

```hcl
data "portainer_docker_config" "app_cfg" {
  endpoint_id = 1
  name        = "app-v1-config"
}

output "config_id" {
  value = data.portainer_docker_config.app_cfg.id
}
```

## Arguments Reference

| Name          | Type    | Required | Description                              |
|---------------|---------|----------|------------------------------------------|
| `endpoint_id` | integer | ✅ yes   | ID of the environment.                  |
| `name`        | string  | ✅ yes   | Name of the Docker config.              |

## Attributes Reference

| Name | Type   | Description                  |
|------|--------|------------------------------|
| `id` | string | ID of the Docker config.     |
