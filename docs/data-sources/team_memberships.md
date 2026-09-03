# Data Source Documentation: `portainer_team_memberships`

# portainer_team_memberships
Lists the members of a team and identifies its leaders.

## Example Usage

```hcl
data "portainer_team_memberships" "platform" {
  team_id = portainer_team.platform.id
}

output "platform_leaders" {
  value = data.portainer_team_memberships.platform.leader_user_ids
}
```

## Arguments Reference

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `team_id` | number | ✅ yes | Identifier of the team whose members are listed. |

## Attributes Reference

| Name | Type | Description |
|------|------|-------------|
| `memberships` | list | Members of the team. Each entry has `id`, `user_id` and `role` (1 = team leader, 2 = team member). |
| `leader_user_ids` | list | Identifiers of the team's leaders, who may manage its membership. |
