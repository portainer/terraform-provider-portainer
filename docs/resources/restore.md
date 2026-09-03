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
| `file_content_base64` | string | ✅ yes   | –                            | Base64-encoded backup archive, typically `filebase64(...)`. Stored in state as a sensitive value.        |
| `file_name`           | string | ❌ no    | `portainer-backup.tar.gz`    | Name reported to Portainer for the archive.                                                              |
| `password`            | string | ❌ no    | –                            | Password the backup was encrypted with. Stored in state as a sensitive value.                            |
| `setup_token`         | string | ❌ no    | –                            | Sent as the `X-Setup-Token` header, which Portainer requires on an uninitialised instance.               |

Every argument forces a new resource: a restore is a one-shot operation with nothing to update.

## Attributes Reference

| Name | Description                                  |
|------|----------------------------------------------|
| `id` | Synthetic identifier carrying a timestamp.   |

**A backup archive contains every credential Portainer holds.** Both the archive and its password end up in Terraform state — treat that state as a secret, or drive the restore from a configuration whose state is stored securely and separately.
