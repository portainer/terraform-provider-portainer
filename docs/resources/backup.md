# 💾 **Resource Documentation: portainer_backup**

# portainer_backup
The `portainer_backup` resource allows you to trigger a backup of your Portainer instance directly via Terraform.
> ⚠️ Note: The backup is saved locally at the path specified by output_path.
The backup is encrypted using the password.

## Example Usage
### Create Backup

```hcl
resource "portainer_backup" "your-backup" {
  password    = "your-backup-password"
  output_path = "your-backup-path-for-tar-gz-file"
}
```

## Lifecycle & Behavior
This resource performs a one-time backup when applied. It does not manage state and will always re-trigger on each terraform apply.

## Arguments Reference

| Name         | Type   | Required | Description                                              |
|--------------|--------|----------|----------------------------------------------------------|
| `password`   | string | ✅ yes   | Password used to encrypt the backup archive.             |
| `output_path`| string | ✅ yes   | Path to store the output backup file (e.g. "backup.tar.gz"). |
