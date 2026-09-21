# Resource Documentation: `portainer_user_memberships_sync`

# portainer_user_memberships_sync

> **Business Edition only.** This endpoint does not exist in Portainer CE.


Refreshes a user's team memberships from the configured LDAP/AD or OAuth provider.

## Example Usage

```hcl
resource "portainer_user_memberships_sync" "example" {
  user_id = portainer_user.alice.id

  triggers = {
    run_at = timestamp()
  }
}
```

## Arguments Reference

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `user_id` | number | ✅ yes | Identifier of the user whose team memberships are synchronised. |
| `triggers` | map | ❌ no | Arbitrary values that force another run when they change. |

This is a one-shot action with no state to read back, so without a `triggers` value the resource runs once and then stays put.
