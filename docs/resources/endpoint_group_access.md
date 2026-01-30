# 🌐 **Resource Documentation: `portainer_endpoint_group_access`**

# portainer_endpoint_group_access
The `portainer_endpoint_group_access` resource allows you to manage access control for a Portainer endpoint group.

## Example Usage

```hcl
# Create a team
resource "portainer_team" "dev_team" {
  name = "Development"
}

# Create an endpoint group
resource "portainer_endpoint_group" "example" {
  name        = "example-group"
  description = "A group for development endpoints"
}

# Assign access to the team for the endpoint group
resource "portainer_endpoint_group_access" "dev_team_access" {
  endpoint_group_id = portainer_endpoint_group.example.id
  team_id           = portainer_team.dev_team.id
  role_id           = 0 # 0 = Standard User (usually implies read-only or standard access depending on global settings)
}

# Assign access to a specific user for the endpoint group
# resource "portainer_endpoint_group_access" "user_access" {
#   endpoint_group_id = portainer_endpoint_group.example.id
#   user_id           = 25
#   role_id           = 0
# }
```

## Arguments Reference

| Name                | Type | Required | Description                                                                        |
| ------------------- | ---- | -------- | ---------------------------------------------------------------------------------- |
| `endpoint_group_id` | int  | ✅ yes    | ID of the Portainer endpoint group                                                 |
| `team_id`           | int  | 🚫 optional | ID of the team to grant access. One of `team_id` or `user_id` must be provided.    |
| `user_id`           | int  | 🚫 optional | ID of the user to grant access. One of `team_id` or `user_id` must be provided.    |
| `role_id`           | int  | 🚫 optional | Access role ID (default `0`).                                                      |

## Attributes Reference

| Name | Description                |
| ---- | -------------------------- |
| `id` | ID of the resource control |
