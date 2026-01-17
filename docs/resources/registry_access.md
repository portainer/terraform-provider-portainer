# 🌐 **Resource Documentation: `portainer_registry_access`**

# portainer_registry_access
The `portainer_registry_access` resource allows you to manage access control for a Portainer registry on a specific environment (endpoint).

## Example Usage

```hcl
# Create a team
resource "portainer_team" "dev_team" {
  name = "Development"
}

# Assign team access to the environment (endpoint)
resource "portainer_environment" "example" {
  name                = "local"
  environment_address = "localhost"
  type                = 1
  team_access_policies = {
    (portainer_team.dev_team.id) = 2 # 2 = Standard User
  }
}

# Define the registry
resource "portainer_registry" "my_registry" {
  name = "my-custom-registry"
  url  = "registry.example.com"
  type = 3 # Custom
}

# Assign the registry access to the team on that environment
resource "portainer_registry_access" "dev_team_access" {
  registry_id = portainer_registry.my_registry.id
  endpoint_id = portainer_environment.example.id
  team_id     = portainer_team.dev_team.id
}
```

## Arguments Reference

| Name          | Type | Required | Description                                                                        |
| ------------- | ---- | -------- | ---------------------------------------------------------------------------------- |
| `registry_id` | int  | ✅ yes    | ID of the Portainer registry                                                       |
| `endpoint_id` | int  | ✅ yes    | ID of the Portainer environment (endpoint)                                         |
| `team_id`     | int  | 🚫 optional | ID of the team to grant access. One of `team_id` or `user_id` must be provided.    |
| `user_id`     | int  | 🚫 optional | ID of the user to grant access. One of `team_id` or `user_id` must be provided.    |
| `role_id`     | int  | 🚫 optional | Access role ID (default `0`).                                                      |

## Attributes Reference

| Name | Description                |
| ---- | -------------------------- |
| `id` | ID of the resource control |
