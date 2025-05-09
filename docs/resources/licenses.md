# 📜 **Resource Documentation: `portainer_licenses`**

# portainer_licenses
The `portainer_licenses` resource allows you to attach license keys to your Portainer instance. This is required for enabling Portainer Business Edition or advanced features that require licensing.

## Example Usage
```hcl
resource "portainer_licenses" "example" {
  key   = "your-license-key-here"
  force = true # optional
}
```
## Lifecycle & Behavior
Apply and update licenses key by:
```sh
terraform apply
```

To remove the licenses key:
```sh
terraform destroy
```

> Setting `force = true` will overwrite any conflicting license keys already registered in Portainer.

## Arguments Reference
| Name   | Type   | Required | Description                                                             |
|--------|--------|----------|-------------------------------------------------------------------------|
| `key`  | string | ✅ yes   | Portainer license key.                                                  |
| `force`| bool   | 🚫 no    | Whether to force remove any conflicting licenses (default: false).      |
