# Resource Documentation: `portainer_settings_default_registry`

# portainer_settings_default_registry

> **Business Edition only.** This endpoint does not exist in Portainer CE.


Controls whether Portainer offers the built-in anonymous Docker Hub registry when a user picks an image source.

## Example Usage

```hcl
resource "portainer_settings_default_registry" "main" {
  hide = true
}
```

## Arguments Reference

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `hide` | bool | ✅ yes | Whether to hide the built-in anonymous Docker Hub registry, so users can only deploy from registries you have configured. |

The setting has its own endpoint rather than being part of `portainer_settings`, which is why it is a separate resource: folding it in would make every settings apply rewrite it too.

Portainer has no endpoint to clear the setting, so destroying the resource stops managing it and leaves the registry as configured. Turning the built-in registry back on is a change to make explicitly.

## Import

```bash
terraform import portainer_settings_default_registry.main portainer-settings-default-registry
```
