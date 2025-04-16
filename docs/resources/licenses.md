# 📜 **Resource Documentation: `portainer_licenses`**

# portainer_licenses
The `portainer_licenses` resource allows you to attach license keys to your Portainer instance. This is required for enabling Portainer Business Edition or advanced features that require licensing.

## Example Usage
```hcl
resource "portainer_licenses" "example" {
  key   = "your-license-key-here"
  force = true
}
```
## Lifecycle & Behavior
Licenses are added using the `/licenses/add` Portainer API endpoint.

Setting `force = true` will overwrite any conflicting license keys already registered in Portainer.

If the provided license key is already active, the resource will still be marked as successfully created.

> ⚠️ **Note:** This resource is **write-only** — it does not support reading or importing existing license keys.

## Arguments Reference
| Name   | Type   | Required | Description                                                             |
|--------|--------|----------|-------------------------------------------------------------------------|
| `key`  | string | ✅ yes   | Portainer license key.                                                  |
| `force`| bool   | 🚫 no    | Whether to force remove any conflicting licenses (default: false).      |
