# 👥📘 **Resource Documentation: `portainer_team_membership`**

# portainer_team_membership
The `portainer_team_membership` resource allows you to assign an existing user to a team with a specific role in Portainer.

## Example Usage
```hcl
resource "portainer_user" "example_user" {
  username  = "exampleuser"
  password  = "SuperStrongPassword123!"
  role      = 2
  ldap_user = false
}

resource "portainer_team" "example_team" {
  name = "dev-team"
}

resource "portainer_team_membership" "membership" {
  role    = 2
  team_id = portainer_team.example_team.id
  user_id = portainer_user.example_user.id
}
```
## Lifecycle & Behavior
Team membrship are updated if any of the attributes change (e.g. role).

- To delete a membrship created via Terraform, simply run:
```hcl
terraform destroy
```

- To change a team membrship role id, update the role field and re-apply:
```hcl
terraform apply
```

## Arguments Reference
| Name     | Type   | Required | Description                                        |
|----------|--------|----------|----------------------------------------------------|
| `role`   | number | ✅ yes   | Role of the user in the team (1 = leader, 2 = member) |
| `team_id`| number | ✅ yes   | ID of the Portainer team                          |
| `user_id`| number | ✅ yes   | ID of the Portainer user                          |

## Attributes Reference
| Name | Description              |
|------|--------------------------|
| `id` | ID of the membership     |
