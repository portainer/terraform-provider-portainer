# 👥📘 **Data Source Documentation: `portainer_team_membership`**

## portainer_team_membership

The `portainer_team_membership` data source allows you to retrieve information about a user's membership in a specific team.

### Example Usage

```hcl
data "portainer_team_membership" "my_membership" {
  team_id = 1
  user_id = 5
}

output "user_role_in_team" {
  value = data.portainer_team_membership.my_membership.role
}
```

---

### Arguments Reference

| Name      | Type | Required | Description        |
|-----------|------|----------|--------------------|
| `team_id` | int  | ✅ yes    | ID of the team.    |
| `user_id` | int  | ✅ yes    | ID of the user.    |

---

### Attributes Reference

| Name   | Description                                                           |
|--------|-----------------------------------------------------------------------|
| `id`   | The target Team Membership ID.                                        |
| `role` | The role of the user within the team (1 for leader, 2 for member).    |
