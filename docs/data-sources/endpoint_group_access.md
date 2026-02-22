# 🌐 **Data Source Documentation: `portainer_endpoint_group_access`**

## portainer_endpoint_group_access

The `portainer_endpoint_group_access` data source allows you to retrieve the access role (policy) of a specific team or user on an endpoint group.

### Example Usage

```hcl
// Querying a team's access to an endpoint group
data "portainer_endpoint_group_access" "team_access" {
  endpoint_group_id = 1
  team_id           = 2
}

output "team_role" {
  value = data.portainer_endpoint_group_access.team_access.role_id
}

// Querying a user's access to an endpoint group
data "portainer_endpoint_group_access" "user_access" {
  endpoint_group_id = 1
  user_id           = 5
}
```

---

### Arguments Reference

| Name                | Type | Required    | Description                              |
|---------------------|------|-------------|------------------------------------------|
| `endpoint_group_id` | int  | ✅ yes       | ID of the endpoint group.                |
| `team_id`           | int  | 🚫 optional | ID of the team (must provide team or user).|
| `user_id`           | int  | 🚫 optional | ID of the user (must provide team or user).|

---

### Attributes Reference

| Name      | Description                                               |
|-----------|-----------------------------------------------------------|
| `id`      | Internal identifier of the access policy.                 |
| `role_id` | The role ID granted to the user or team (e.g. 1, 2, 3, etc).|
