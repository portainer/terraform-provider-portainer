# 💾☁️ **Resource Documentation: portainer_backup_s3**

# portainer_backup_s3
The `portainer_backup_s3` resource allows you to back up your Portainer instance to an AWS S3 bucket or compatible storage system.
> ⚠️ Note: This resource performs a one-time backup upload on every terraform apply. It is not stateful and will always re-trigger.

> Currently working only for Portainer BE edition

## Example Usage

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
- [Example on GitHub](https://github.com/portainer/terraform-provider-portainer/tree/main/examples/backup_s3)

## Lifecycle & Behavior
This resource does not track state — it performs a one-time backup to the specified S3 bucket every time terraform apply runs.

## Arguments Reference

| Name                 | Type   | Required | Description                                                                   |
|----------------------|--------|----------|-------------------------------------------------------------------------------|
| `access_key_id`      | string | ✅ yes   | AWS access key ID or S3-compatible access key                                 |
| `secret_access_key`  | string | ✅ yes   | AWS secret key or S3-compatible secret key                                    |
| `bucket_name`        | string | ✅ yes   | Name of the S3 bucket where backup should be stored                           |
| `region`             | string | ✅ yes   | AWS region (e.g., `eu-central-1`)                                             |
| `s3_compatible_host` | string | ✅ yes   | Hostname of S3-compatible provider (e.g., `https://s3.example.com`)           |
| `password`           | string | ✅ yes   | Password to encrypt the backup archive                                        |
| `cron_rule`          | string | 🚫 optional | Optional cron rule for scheduling backups (e.g., `@daily`) *(not yet stored in state)* |
