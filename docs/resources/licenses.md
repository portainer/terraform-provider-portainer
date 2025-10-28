# 📜 **Resource Documentation: `portainer_licenses`**

# portainer_licenses
The `portainer_licenses` resource allows you to attach license keys to your Portainer instance. This is required for enabling Portainer Business Edition or advanced features that require licensing.

> Currently working only for Portainer BE edition

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
| Name               | Type         | Required    | Description                                                                        |
| ------------------ | ------------ | ----------- | ---------------------------------------------------------------------------------- |
| `key`              | string       | ✅ yes      | License key to be attached. Sensitive and immutable.                               |
| `force`            | bool         | 🚫 optional | Whether to force attach even if there are conflicting licenses (default: `false`). |
| `conflicting_keys` | list(string) | 🚫 optional | List of conflicting license keys, if any were detected.                            |
