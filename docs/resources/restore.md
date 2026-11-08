# Resource Documentation: `portainer_restore`

# portainer_restore
The `portainer_restore` resource restores a Portainer instance from a backup archive, the counterpart of `portainer_backup`.

A restore replaces the instance's entire database. Portainer only accepts it on an instance that has **not been initialised yet**, which is why `setup_token` exists — so this belongs in a bootstrap configuration, not alongside resources that manage a running instance.

## Example Usage

```hcl
resource "portainer_restore" "bootstrap" {
  file_content_base64 = filebase64("portainer-backup.tar.gz")
  file_name           = "portainer-backup.tar.gz"
  password            = var.backup_password
  setup_token         = var.setup_token
}
```

## Arguments Reference

| Name                  | Type   | Required | Default                      | Description                                                                                          |
|-----------------------|--------|----------|------------------------------|--------------------------------------------------------------------------------------------------------|
| `file_content_base64` | string | ❌ no | –                            | Base64-encoded backup archive, typically `filebase64(...)`. Stored in state as a sensitive value. One of `file_content_base64` or `file_content_base64_wo` is required. |
| `file_content_base64_wo` | string | ❌ no | – | Write-only `file_content_base64` (supports ephemeral values; never stored in Terraform state; Terraform 1.11+). Conflicts with `file_content_base64`; requires `file_content_base64_wo_version`. |
| `file_content_base64_wo_version` | number | ❌ no | – | Version of `file_content_base64_wo`; increment it to send a new value. Changing it forces resource recreation. |
| `file_name`           | string | ❌ no    | `portainer-backup.tar.gz`    | Name reported to Portainer for the archive.                                                              |
| `password`            | string | ❌ no    | –                            | Password the backup was encrypted with. Stored in state as a sensitive value.                            |
| `password_wo` | string | ❌ no | – | Write-only `password` (supports ephemeral values; never stored in Terraform state; Terraform 1.11+). Conflicts with `password`; requires `password_wo_version`. |
| `password_wo_version` | number | ❌ no | – | Version of `password_wo`; increment it to send a new value. Changing it forces resource recreation. |
| `setup_token`         | string | ❌ no    | –                            | Sent as the `X-Setup-Token` header, which Portainer requires on an uninitialised instance.               |
| `setup_token_wo` | string | ❌ no | – | Write-only `setup_token` (supports ephemeral values; never stored in Terraform state; Terraform 1.11+). Conflicts with `setup_token`; requires `setup_token_wo_version`. |
| `setup_token_wo_version` | number | ❌ no | – | Version of `setup_token_wo`; increment it to send a new value. Changing it forces resource recreation. |

Every argument forces a new resource: a restore is a one-shot operation with nothing to update.

## Attributes Reference

| Name | Description                                  |
|------|----------------------------------------------|
| `id` | Synthetic identifier carrying a timestamp.   |

**A backup archive contains every credential Portainer holds.** Both the archive and its password end up in Terraform state — treat that state as a secret, or drive the restore from a configuration whose state is stored securely and separately.
