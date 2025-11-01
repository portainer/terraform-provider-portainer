# 💾☁️ **Resource Documentation: portainer_backup_s3**

# portainer_backup_s3
The `portainer_backup_s3` resource allows you to back up your Portainer instance to an AWS S3 bucket or compatible storage system.
> ⚠️ Note: This resource performs a one-time backup upload on every terraform apply. It is not stateful and will always re-trigger.

> Currently working only for Portainer BE edition

## Example Usage
### Standart
- [Example on GitHub](https://github.com/portainer/terraform-provider-portainer/tree/main/examples/backup_s3)

```hcl
resource "portainer_backup_s3" "s3_backup" {
  access_key_id      = "s3_access_key"
  secret_access_key  = "s3_secret_key"
  bucket_name        = "s3_bucket"
  region             = "s3_region"
  s3_compatible_host = "s3_endpoint"
  password           = "backup_password"
  cron_rule          = var.backup_cron_rule   # optional
}
```

### Example Usage (Ephemeral Credentials)

This variant avoids storing sensitive values in Terraform state.
All `_wo` (write-only) attributes are ephemeral and will **only be used at apply time**.

```hcl
resource "portainer_backup_s3" "s3_backup_ephemeral" {
  bucket_name        = "portainer-backups"
  region             = "eu-central-1"
  s3_compatible_host = "https://s3.eu-central-1.amazonaws.com"

  # Ephemeral (write-only) credentials
  access_key_id_wo     = var.s3_access_key_id
  secret_access_key_wo = var.s3_secret_access_key
  password_wo          = var.s3_backup_password

  # Increase this version to force recreation after credential rotation
  backup_wo_version = 1

  cron_rule = "@daily"
}
```

## Lifecycle & Behavior
This resource does not track state — it performs a one-time backup to the specified S3 bucket every time terraform apply runs.

## Arguments Reference

| Name                   | Type   | Required    | Description                                                                   |
| ---------------------- | ------ | ----------- | ----------------------------------------------------------------------------- |
| `access_key_id`        | string | ✅ yes*      | AWS or S3-compatible access key (stored in state)                             |
| `secret_access_key`    | string | ✅ yes*      | AWS or S3-compatible secret key (stored in state)                             |
| `password`             | string | ✅ yes*      | Password used to encrypt the backup archive (stored in state)                 |
| `access_key_id_wo`     | string | 🚫 optional | **Write-only** S3 access key (ephemeral, not stored in Terraform state)       |
| `secret_access_key_wo` | string | 🚫 optional | **Write-only** S3 secret key (ephemeral, not stored in Terraform state)       |
| `password_wo`          | string | 🚫 optional | **Write-only** encryption password (ephemeral, not stored in Terraform state) |
| `backup_wo_version`    | int    | 🚫 optional | Version flag to trigger new backup when `_wo` credentials are rotated         |
| `bucket_name`          | string | ✅ yes       | Target S3 bucket name where the backup will be uploaded                       |
| `region`               | string | ✅ yes       | AWS region (e.g. `eu-central-1`)                                              |
| `s3_compatible_host`   | string | ✅ yes       | Hostname or endpoint of S3-compatible provider                                |
| `cron_rule`            | string | 🚫 optional | Optional cron syntax for scheduling backups (not stateful yet)                |

> ⚠️ You must use either the standard credentials **or** the `_wo` (write-only) variant — never both at once.
