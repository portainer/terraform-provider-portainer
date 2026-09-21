# Data Source Documentation: `portainer_addons`

# portainer_addons

> **Business Edition only.** This endpoint does not exist in Portainer CE.


Lists the addons the connected Portainer offers, and reports the environment addon resources are served from.

## Example Usage

```hcl
data "portainer_addons" "all" {}

output "upgradable" {
  value = [for a in data.portainer_addons.all.addons : a.id if a.upgrade_available]
}
```

## Arguments Reference

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `view` | string | ❌ no | Set to `switcher` for the lightweight app-switcher listing, which omits the catalog details and is readable by non-admin users. |

## Attributes Reference

| Name | Type | Description |
|------|------|-------------|
| `environment_id` | number | Environment addon resources are read through. |
| `catalog_error` | string | Why the catalog could not be refreshed. Admin-only, and never set on the `switcher` view. |
| `addons` | list(object) | The addons on offer. |

### `addons`

| Name | Type | Description |
|------|------|-------------|
| `id` | string | Catalog identifier of the addon. |
| `display_name` | string | Human-readable name. |
| `description` | string | Full catalog description. |
| `short_description` | string | One-line catalog tagline. |
| `icon` | string | Icon the Portainer UI shows. |
| `path` | string | Path the addon is served under. |
| `enabled` | bool | Whether the addon is installed and switched on. |
| `lifecycle_status` | string | Helm release state, empty when the addon was never installed. |
| `lifecycle_status_message` | string | Explanation of a failed install or upgrade. |
| `health_status` | string | Live probe result. |
| `health_message` | string | Explanation of an unhealthy probe result. |
| `chart_version` | string | Chart version currently installed. |
| `upgrade_available` | bool | Whether a newer chart version has been published. |
| `available_versions` | list(string) | Published stable chart versions, newest first. |

When `catalog_error` is set and addons are still listed, the listing is a cached copy; when it is set and the list is empty, nothing could be loaded. The version and chart fields come from an admin-only part of the response, so they stay empty for a token that cannot see it.
