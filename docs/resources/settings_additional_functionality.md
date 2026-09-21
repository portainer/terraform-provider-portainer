# Resource Documentation: `portainer_settings_additional_functionality`

# portainer_settings_additional_functionality

> **Business Edition only.** This endpoint does not exist in Portainer CE.


Switches optional Portainer features on and off. Today that is the policy engine, which gates the `portainer_policy` resources.

## Example Usage

```hcl
resource "portainer_settings_additional_functionality" "main" {
  policies = true
}

resource "portainer_policy" "baseline" {
  depends_on = [portainer_settings_additional_functionality.main]
  # ...
}
```

## Arguments Reference

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `policies` | bool | ✅ yes | Whether the policy engine is enabled. |

Destroying the resource stops managing the setting rather than turning the feature off, which would break every policy resource that depends on it.

## Import

```bash
terraform import portainer_settings_additional_functionality.main portainer-settings-additional-functionality
```
