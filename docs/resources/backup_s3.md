# 💾☁️ **Resource Documentation: portainer_backup_s3**

# portainer_backup_s3
The `portainer_backup_s3` resource allows you to back up your Portainer instance to an AWS S3 bucket or compatible storage system.
> ⚠️ Note: This resource performs a one-time backup upload on every terraform apply. It is not stateful and will always re-trigger.

## Example Usage
### Create Backup

```hcl
resource "portainer_backup" "your-backup" {
  password    = "your-backup-password"
  output_path = "your-backup-path-for-tar-gz-file"
}
```

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
