# Resource Documentation: `portainer_addon_config`

# portainer_addon_config

> **Business Edition only.** This endpoint does not exist in Portainer CE.


Manages the configuration entries an addon reads at runtime, for example a base domain or an API token.

## Example Usage

```hcl
resource "portainer_addon_config" "portal" {
  addon_id = portainer_addon.portal.addon_id

  entry {
    key   = "BASE_DOMAIN"
    value = "apps.example.com"
  }

  entry {
    key       = "API_TOKEN"
    value     = var.portal_api_token
    sensitive = true
  }
}
```

## Arguments Reference

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `addon_id` | string | ✅ yes | Catalog identifier of the addon. Changing it forces a new resource. |
| `entry` | block | ✅ yes | Configuration entries of the addon. Repeatable. |

### `entry`

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `key` | string | ✅ yes | Name of the entry, for example `BASE_DOMAIN`. |
| `value` | string | ✅ yes | Value of the entry. Always treated as sensitive by Terraform, so it is not shown in a plan. |
| `sensitive` | bool | ❌ no | Whether Portainer should treat the value as a secret. Defaults to `false`. |

Creating the resource replaces the addon's stored configuration, which is what taking ownership of it means. Subsequent applies work entry by entry, so a key added outside Terraform is left alone rather than destroyed on every run. Destroying the resource clears the configuration.

Portainer does not return the value of an entry marked `sensitive`. The provider keeps the configured value in state rather than blanking it, which is what keeps a plan from showing a change on every run - but it also means drift in a sensitive value cannot be detected.

## Import

```bash
terraform import portainer_addon_config.portal portal-template
```

The import ID is the addon's catalog identifier. Sensitive values are not returned by Portainer, so they have to be supplied in the configuration after an import.
