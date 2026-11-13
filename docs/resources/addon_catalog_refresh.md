# Resource Documentation: `portainer_addon_catalog_refresh`

# portainer_addon_catalog_refresh

> **Requires Portainer Business Edition 2.45.2 or later.** On Community Edition or an older server the refresh fails with an error that says so.

Drops Portainer's cached addon catalog, so the next read loads it from the configured catalog source. The last good copy is kept and still serves when that load fails. Requires an administrator.

## Example Usage

```hcl
resource "portainer_addon_catalog_refresh" "now" {
  triggers = {
    # Refresh again whenever the catalog is expected to have changed.
    catalog_release = "2026-11"
  }
}

data "portainer_addon_catalog" "catalog" {
  depends_on = [portainer_addon_catalog_refresh.now]
}
```

## Arguments Reference

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `triggers` | map | ❌ no | Arbitrary values that force another refresh when they change. |

A refresh is a one-shot action, so without a `triggers` value the resource runs once and then stays put. Destroying it does nothing.
