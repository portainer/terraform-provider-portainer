# Data Source Documentation: `portainer_role`

# portainer_role
The `portainer_role` data source allows you to list all available Portainer roles or look up a specific role by name.

## Example Usage

### List all roles

```hcl
data "portainer_role" "all" {}

output "roles" {
  value = data.portainer_role.all.roles
}
```

### Look up a specific role by name

```hcl
data "portainer_role" "helpdesk" {
  name = "HelpDesk"
}

output "helpdesk_role_id" {
  value = data.portainer_role.helpdesk.roles[0].id
}
```

## Arguments Reference

| Name   | Type   | Required | Description                                                |
|--------|--------|----------|------------------------------------------------------------|
| `name` | string | No       | Filter by role name. If set, only the matching role is returned. |

## Attributes Reference

| Name    | Type | Description                                                            |
|---------|------|------------------------------------------------------------------------|
| `roles` | list | List of roles. Each entry has `id`, `name`, `description`, `priority`. |
| `id`    | string | When filtering by name, set to the matching role's ID.               |
