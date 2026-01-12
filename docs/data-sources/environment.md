# 🌐 **Data Source Documentation: `portainer_environment`**

# portainer_environment
The `portainer_environment` data source (referred to as "endpoint" in the Portainer API) allows you to look up an existing Portainer environment by its name. This is essential for retrieving the ID needed to deploy stacks or configure access control.

## Example Usage

### Look up an environment by name

```hcl
data "portainer_environment" "local" {
  name = "local"
}

output "environment_id" {
  value = data.portainer_environment.local.id
}
```

## Arguments Reference

| Name   | Type   | Required | Description                     |
|--------|--------|----------|---------------------------------|
| `name` | string | ✅ yes   | Name of the Portainer environment. |

## Attributes Reference

| Name                  | Type    | Description                                                                 |
|-----------------------|---------|-----------------------------------------------------------------------------|
| `id`                  | string  | ID of the Portainer environment.                                             |
| `type`                | integer | Environment type: `1`=Docker, `2`=Agent, `3`=Azure, `4`=EdgeAgent, `5`=K8s. |
| `environment_address` | string  | The URL/address of the environment.                                         |
| `group_id`            | integer | ID of the group this environment belongs to.                               |
