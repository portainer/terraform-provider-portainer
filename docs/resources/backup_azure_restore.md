# Resource Documentation: `portainer_backup_azure_restore`

# portainer_backup_azure_restore

> **Business Edition only.** This endpoint does not exist in Portainer CE.


Restores a Portainer instance from an archive in Azure Blob Storage.

## Example Usage

```hcl
resource "portainer_backup_azure_restore" "recover" {
  auth_method          = "storageAccountKey"
  storage_account_name = "acmebackups"
  container_name       = "portainer"
  blob_name            = "portainer-2026-09-15.tar.gz"
  storage_account_key  = var.azure_storage_key
  password             = var.backup_archive_password
}
```

## Arguments Reference

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `auth_method` | string | ✅ yes | Azure authentication method. |
| `storage_account_name` | string | ✅ yes | Storage account holding the container. |
| `container_name` | string | ✅ yes | Container holding the archive. |
| `blob_name` | string | ✅ yes | Blob to restore from. |
| `service_url` | string | ❌ no | Blob service endpoint, for a sovereign cloud or an emulator. |
| `storage_account_key` | string | ❌ no | Storage account key. Used with `storageAccountKey`. Sensitive. |
| `storage_account_key_wo` | string | ❌ no | Write-only `storage_account_key` (supports ephemeral values; never stored in Terraform state; Terraform 1.11+). Conflicts with `storage_account_key`; requires `storage_account_key_wo_version`. |
| `storage_account_key_wo_version` | number | ❌ no | Version of `storage_account_key_wo`; increment it to send a new value. Changing it forces resource recreation. |
| `tenant_id` | string | ❌ no | Entra ID tenant of the service principal. |
| `client_id` | string | ❌ no | Application (client) ID of the service principal. |
| `client_secret` | string | ❌ no | Client secret. Used with `servicePrincipalSecret`. Sensitive. |
| `client_secret_wo` | string | ❌ no | Write-only `client_secret` (supports ephemeral values; never stored in Terraform state; Terraform 1.11+). Conflicts with `client_secret`; requires `client_secret_wo_version`. |
| `client_secret_wo_version` | number | ❌ no | Version of `client_secret_wo`; increment it to send a new value. Changing it forces resource recreation. |
| `client_certificate` | string | ❌ no | PEM client certificate. Used with `servicePrincipalCertificate`. Sensitive. |
| `client_certificate_wo` | string | ❌ no | Write-only `client_certificate` (supports ephemeral values; never stored in Terraform state; Terraform 1.11+). Conflicts with `client_certificate`; requires `client_certificate_wo_version`. |
| `client_certificate_wo_version` | number | ❌ no | Version of `client_certificate_wo`; increment it to send a new value. Changing it forces resource recreation. |
| `client_certificate_password` | string | ❌ no | Password protecting `client_certificate`. Sensitive. |
| `client_certificate_password_wo` | string | ❌ no | Write-only `client_certificate_password` (supports ephemeral values; never stored in Terraform state; Terraform 1.11+). Conflicts with `client_certificate_password`; requires `client_certificate_password_wo_version`. |
| `client_certificate_password_wo_version` | number | ❌ no | Version of `client_certificate_password_wo`; increment it to send a new value. Changing it forces resource recreation. |
| `managed_identity_client_id` | string | ❌ no | Client ID of a user-assigned managed identity; unset means system-assigned. |
| `password` | string | ❌ no | Password the archive itself is encrypted with. Sensitive. |
| `password_wo` | string | ❌ no | Write-only `password` (supports ephemeral values; never stored in Terraform state; Terraform 1.11+). Conflicts with `password`; requires `password_wo_version`. |
| `password_wo_version` | number | ❌ no | Version of `password_wo`; increment it to send a new value. Changing it forces resource recreation. |

## Attributes Reference

| Name | Type | Description |
|------|------|-------------|
| `id` | string | Synthetic identifier carrying a timestamp. |

A restore replaces the instance's entire database, so it belongs in a bootstrap configuration rather than alongside resources managing a running instance. Every argument forces a new resource.
