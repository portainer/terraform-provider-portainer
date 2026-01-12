# 🔑 **Data Source Documentation: `portainer_docker_secret`**

# portainer_docker_secret
The `portainer_docker_secret` data source allows you to look up an existing Docker secret within a specific Portainer Swarm environment.

## Example Usage

### Look up a Docker secret by name

```hcl
data "portainer_docker_secret" "db_pwd" {
  endpoint_id = 1
  name        = "db-password"
}

output "secret_id" {
  value = data.portainer_docker_secret.db_pwd.id
}
```

## Arguments Reference

| Name          | Type    | Required | Description                              |
|---------------|---------|----------|------------------------------------------|
| `endpoint_id` | integer | ✅ yes   | ID of the environment.                  |
| `name`        | string  | ✅ yes   | Name of the Docker secret.              |

## Attributes Reference

| Name | Type   | Description                  |
|------|--------|------------------------------|
| `id` | string | ID of the Docker secret.     |
