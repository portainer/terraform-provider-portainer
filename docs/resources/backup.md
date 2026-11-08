# 💾 **Resource Documentation: portainer_backup**

# portainer_backup
The `portainer_backup` resource allows you to trigger a backup of your Portainer instance directly via Terraform.
> ⚠️ Note: The backup is saved locally at the path specified by output_path.
The backup is encrypted using the password.

## Example Usage
```hcl
resource "portainer_backup" "backup" {
  password    = "your-backup-password"
  output_path = "backup.tar.gz"
}
```
- [Example on GitHub](https://github.com/portainer/terraform-provider-portainer/tree/main/examples/backup)

## Lifecycle & Behavior
This resource performs a one-time backup when applied. It does not manage state and will always re-trigger on each terraform apply.

## Arguments Reference

| Name         | Type   | Required | Description                                              |
|--------------|--------|----------|----------------------------------------------------------|
| `password`   | string | no | Password used to encrypt the backup archive. One of `password` or `password_wo` is required. |
| `password_wo` | string | no | Write-only `password` (supports ephemeral values; never stored in Terraform state; Terraform 1.11+). Conflicts with `password`; requires `password_wo_version`. |
| `password_wo_version` | number | no | Version of `password_wo`; increment it to send a new value. Changing it forces resource recreation. |
| `output_path`| string | ✅ yes   | Path to store the output backup file (e.g. "backup.tar.gz"). |
