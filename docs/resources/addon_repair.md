# Resource Documentation: `portainer_addon_repair`

# portainer_addon_repair

> **Business Edition only.** This endpoint does not exist in Portainer CE.


Reissues the credential an installed addon authenticates to Portainer with. Run it when an addon reports `health_status = "credential-invalid"`.

## Example Usage

```hcl
resource "portainer_addon_repair" "portal" {
  addon_id = portainer_addon.portal.addon_id

  triggers = {
    health = portainer_addon.portal.health_status
  }
}
```

## Arguments Reference

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `addon_id` | string | ✅ yes | Catalog identifier of the addon to repair. Changing it forces a new resource. |
| `triggers` | map | ❌ no | Arbitrary values that force another repair when they change. |

## Attributes Reference

| Name | Type | Description |
|------|------|-------------|
| `enabled` | bool | Whether the addon is enabled, as reported after the repair. |
| `chart_version` | string | Chart version of the addon, as reported after the repair. |
| `lifecycle_status` | string | Helm release state of the addon, as reported after the repair. |
| `lifecycle_status_message` | string | Explanation of a failed lifecycle operation. |

A repair is a one-shot action, so without a `triggers` value the resource runs once and then stays put.
