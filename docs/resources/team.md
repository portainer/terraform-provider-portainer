# 👥 **Resource Documentation: `portainer_team`**

# portainer_team
The `portainer_team` resource allows you to manage teams in Portainer.

## Example Usage

### Create Team

```hcl
resource "portainer_team" "your-team" {
  name = "your-team"
}
```

### Create User in Team
```hcl
resource "portainer_team" "your-team" {
  name = "your-team"
}

resource "portainer_user" "your-user" {
  username = "youruser"
  password = "supersecurepassword"
  role     = 2
  team_id   = portainer_team.your-team.id
}
```
## Lifecycle & Behavior

Users are updated if any of the attributes change (e.g. name).

- To delete a team created via Terraform, simply run:
```hcl
terraform destroy
```

- To change a name of team, update the role field and re-apply:
```hcl
terraform apply
```

## Arguments Reference

| Name        | Type    | Required                  | Description                                                                 |
|-------------|---------|---------------------------|-----------------------------------------------------------------------------|
| `name`      | string  | ✅ yes                    | Name of the Portainer team to create.                                       |

## Attributes Reference

| Name | Description              |
|------|--------------------------|
| `id` | ID of the Portainer team |
