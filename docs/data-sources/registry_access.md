# 📡 **Data Source Documentation: `portainer_registry_access`**

## portainer_registry_access

The `portainer_registry_access` data source allows you to retrieve the access role (policy) of a specific team or user on a registry within a specific endpoint.

### Example Usage

```hcl
// Querying a team's access to a registry
data "portainer_registry_access" "team_access" {
  registry_id = 3
  endpoint_id = 1
  team_id     = 2
}

output "team_role" {
  value = data.portainer_registry_access.team_access.role_id
}

// Querying a user's access to a registry
data "portainer_registry_access" "user_access" {
  registry_id = 3
  endpoint_id = 1
  user_id     = 5
}
```

---

### Arguments Reference

| Name          | Type | Required    | Description                              |
|---------------|------|-------------|------------------------------------------|
| `registry_id` | int  | ✅ yes       | ID of the registry.                      |
| `endpoint_id` | int  | ✅ yes       | ID of the endpoint.                      |
| `team_id`     | int  | 🚫 optional | ID of the team (must provide team or user).|
| `user_id`     | int  | 🚫 optional | ID of the user (must provide team or user).|

---

### Attributes Reference

| Name      | Description                                               |
|-----------|-----------------------------------------------------------|
| `id`      | Internal identifier of the access policy.                 |
| `role_id` | The role ID granted to the user or team (e.g. 1, 2, 3, etc).|
