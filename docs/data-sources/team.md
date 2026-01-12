# 👥 **Data Source Documentation: `portainer_team`**

# portainer_team
The `portainer_team` data source allows you to look up an existing Portainer team by its name.

## Example Usage

### Look up a team by name

```hcl
data "portainer_team" "developers" {
  name = "Developers"
}

output "team_id" {
  value = data.portainer_team.developers.id
}
```

## Arguments Reference

| Name   | Type   | Required | Description               |
|--------|--------|----------|---------------------------|
| `name` | string | ✅ yes   | Name of the Portainer team. |

## Attributes Reference

| Name | Type   | Description                |
|------|--------|----------------------------|
| `id` | string | ID of the Portainer team.  |
