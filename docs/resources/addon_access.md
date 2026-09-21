# Resource Documentation: `portainer_addon_access`

# portainer_addon_access

> **Business Edition only.** This endpoint does not exist in Portainer CE.


Owns the access configuration of one addon: whether it is switched on, and which users and teams may use it.

Portainer replaces an addon's access configuration wholesale on every write, so this resource is authoritative for the addon it names. Declaring two of them for the same addon makes each apply revoke the other's grants.

## Example Usage

```hcl
resource "portainer_addon_access" "portal" {
  addon_id = portainer_addon.portal.addon_id
  enabled  = true

  user_access {
    user_id = portainer_user.alice.id
    role_id = 2
  }

  team_access {
    team_id = portainer_team.platform.id
    role_id = 1
  }
}
```

## Arguments Reference

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `addon_id` | string | ✅ yes | Catalog identifier of the addon. Changing it forces a new resource. |
| `enabled` | bool | ❌ no | Whether the addon is switched on. Defaults to `true`. |
| `user_access` | block | ❌ no | Users granted access. Repeatable. |
| `team_access` | block | ❌ no | Teams granted access. Repeatable. |

### `user_access`

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `user_id` | number | ✅ yes | Identifier of the user granted access. |
| `role_id` | number | ✅ yes | Identifier of the role the user is granted. |

### `team_access`

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `team_id` | number | ✅ yes | Identifier of the team granted access. |
| `role_id` | number | ✅ yes | Identifier of the role the team is granted. |

Both blocks are authoritative: a user or team missing from the configuration has no grant. Destroying the resource revokes every grant it manages but leaves `enabled` as it stands, since giving up managing who may use an addon is a different decision than switching the addon off.

## Import

```bash
terraform import portainer_addon_access.portal portal-template
```

The import ID is the addon's catalog identifier.
