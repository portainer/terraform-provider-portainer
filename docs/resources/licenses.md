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

To keep the key out of Terraform state, pass it write-only (Terraform 1.11+):

```hcl
resource "portainer_licenses" "example" {
  key_wo         = var.license_key
  key_wo_version = 1
}
```

The resource ID is the SHA-256 of the key, so an imported license or one attached with `key_wo` is still found and removed on destroy.

## Arguments Reference
| Name               | Type         | Required    | Description                                                                        |
| ------------------ | ------------ | ----------- | ---------------------------------------------------------------------------------- |
| `key`              | string       | 🚫 optional | License key to be attached. Sensitive and immutable. One of `key` or `key_wo` is required. |
| `key_wo`           | string       | 🚫 optional | Write-only `key` (supports ephemeral values; never stored in Terraform state; Terraform 1.11+). Conflicts with `key`; requires `key_wo_version`. |
| `key_wo_version`   | number       | 🚫 optional | Version of `key_wo`; increment it to attach a new key. Changing it forces resource recreation. |
| `force`            | bool         | 🚫 optional | Whether to force attach even if there are conflicting licenses (default: `false`). |
| `conflicting_keys` | list(string) | 🚫 optional | List of conflicting license keys, if any were detected.                            |

## Import

Licenses can be imported using their resource ID, which is the SHA-256 hex digest of the license key:

```shell
terraform import portainer_licenses.example 9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08
```
