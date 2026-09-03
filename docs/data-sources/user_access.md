# Data Source Documentation: `portainer_user_access`

# portainer_user_access
The `portainer_user_access` data source answers "what can this person actually see, and why". It resolves a user's team memberships and their **effective** access on every environment, with the policy each grant comes from — user policy, team policy, or inheritance from an environment group.

With no `user_id` it describes the account the provider authenticates as, which is how a configuration discovers whose API key it is holding.

## Example Usage

### Audit an administrator's reach

```hcl
data "portainer_user_access" "alice" {
  user_id = 8
}

output "alice_environments" {
  value = [
    for a in data.portainer_user_access.alice.effective_access :
    "${a.endpoint_name} — ${a.role_name} (via ${a.access_location})"
  ]
}
```

### Check which account Terraform is using

```hcl
data "portainer_user_access" "me" {}

output "terraform_runs_as" {
  value = data.portainer_user_access.me.username
}
```

## Arguments Reference

| Name      | Type   | Required | Description                                                                       |
|-----------|--------|----------|-----------------------------------------------------------------------------------|
| `user_id` | number | ❌ no    | User to inspect. Leave unset to describe the user the provider authenticates as.   |

## Attributes Reference

| Name               | Type   | Description                                                                                                                                                   |
|--------------------|--------|-----------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `username`         | string | Username of the inspected user.                                                                                                                                 |
| `role`             | number | Portainer role: 1 = administrator, 2 = standard user.                                                                                                            |
| `memberships`      | list   | Team memberships. Each entry has `id`, `team_id` and `role` (1 = team leader, 2 = team member).                                                                   |
| `effective_access` | list   | Effective access per environment. Each entry has `endpoint_id`, `endpoint_name`, `group_id`, `group_name`, `team_id`, `team_name`, `role_id`, `role_name`, `role_priority` and `access_location`. |

`role_priority` is how Portainer resolves a conflict when several grants apply to the same environment, and `access_location` names which of them won.
