# 👤 **Data Source Documentation: `portainer_user`**

# portainer_user
The `portainer_user` data source allows you to look up an existing Portainer user by their username. This is particularly useful for retrieving the ID of users who were auto-provisioned via SSO (Azure AD, OAuth, LDAP).

## Example Usage

### Look up a user by username

```hcl
data "portainer_user" "azure_user" {
  username = "employee@mycompany.onmicrosoft.com"
}

output "user_id" {
  value = data.portainer_user.azure_user.id
}
```

### Assign an SSO user to a team

```hcl
data "portainer_user" "azure_user" {
  username = "employee@mycompany.onmicrosoft.com"
}

data "portainer_team" "developers" {
  name = "Developers"
}

resource "portainer_team_membership" "assign_dev" {
  team_id = data.portainer_team.developers.id
  user_id = data.portainer_user.azure_user.id
  role    = 2 # member
}
```

## Arguments Reference

| Name       | Type   | Required | Description            |
|------------|--------|----------|------------------------|
| `username` | string | ✅ yes   | Username of the user.  |

## Attributes Reference

| Name   | Type    | Description                |
|--------|---------|----------------------------|
| `id`   | string  | ID of the Portainer user.  |
| `role` | integer | Role of the user. `1` = admin, `2` = standard user. |
