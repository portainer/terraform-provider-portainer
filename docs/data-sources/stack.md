# 🥞 **Data Source Documentation: `portainer_stack`**

# portainer_stack
The `portainer_stack` data source allows you to look up an existing Portainer stack by its name within a specific environment.

## Example Usage

### Look up a stack by name

```hcl
data "portainer_environment" "local" {
  name = "local"
}

data "portainer_stack" "nginx" {
  name        = "nginx-stack"
  endpoint_id = data.portainer_environment.local.id
}

output "stack_id" {
  value = data.portainer_stack.nginx.id
}
```

## Arguments Reference

| Name          | Type    | Required | Description                           |
|---------------|---------|----------|---------------------------------------|
| `name`        | string  | ✅ yes   | Name of the stack.                    |
| `endpoint_id` | integer | ✅ yes   | ID of the environment where the stack is deployed. |

## Attributes Reference

| Name       | Type    | Description                                      |
|------------|---------|--------------------------------------------------|
| `id`       | string  | ID of the Portainer stack.                      |
| `type`     | integer | Stack type: `1`=Swarm, `2`=Compose, `3`=K8s. |
| `swarm_id` | string  | ID of the swarm (if applicable).                 |
