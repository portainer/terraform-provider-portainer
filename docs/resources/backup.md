# 💾 **Resource Documentation: portainer_backup**

# portainer_backup
The `portainer_backup` resource allows you to trigger a backup of your Portainer instance directly via Terraform.
> ⚠️ Note: The backup is saved locally at the path specified by output_path.
The backup is encrypted using the password.

## Example Usage
### Standart
```hcl
resource "portainer_backup" "backup" {
  password    = "your-backup-password"
  output_path = "backup.tar.gz"
}
```
- [Example on GitHub](https://github.com/portainer/terraform-provider-portainer/tree/main/examples/backup)

### With write-only varaibles
```hcl
resource "portainer_backup" "secure_local_backup" {
  output_path         = "/tmp/portainer-backup.tar.gz"

  # Ephemeral (write-only) password
  password_wo         = var.backup_password
  backup_wo_version   = 2
}
```

## Lifecycle & Behavior
This resource performs a one-time backup when applied. It does not manage state and will always re-trigger on each terraform apply.

## Arguments Reference

| Name                    | Type   | Required    | Description                                                                                                                    |
| ----------------------- | ------ | ----------- | ------------------------------------------------------------------------------------------------------------------------------ |
| `password`              | string | ✅ yes       | Password used to encrypt the backup archive (stored in Terraform state).                                                       |
| **`password_wo`**       | string | 🚫 optional | **Write-only** password used for backup encryption — **not stored** in Terraform state, only available at apply time.          |
| **`backup_wo_version`** | int    | 🚫 optional | **Write-only** version flag used to trigger recreation when rotating the ephemeral password.                                   |
| `output_path`           | string | ✅ yes       | Local filesystem path where the generated backup archive (`.tar.gz`) will be saved (e.g., `"backup/portainer-backup.tar.gz"`). |
