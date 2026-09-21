# Resource Documentation: `portainer_backup_azure_settings`

# portainer_backup_azure_settings

> **Business Edition only.** This endpoint does not exist in Portainer CE.


Manages Portainer's scheduled backup to Azure Blob Storage: where the archives go, how they authenticate, and on what schedule.

## Example Usage

```hcl
resource "portainer_backup_azure_settings" "nightly" {
  auth_method          = "servicePrincipalSecret"
  storage_account_name = "acmebackups"
  container_name       = "portainer"

  tenant_id     = var.azure_tenant_id
  client_id     = var.azure_client_id
  client_secret = var.azure_client_secret

  cron_rule = "0 2 * * *"
  password  = var.backup_archive_password
}
```

## Arguments Reference

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `auth_method` | string | ✅ yes | How Portainer authenticates against Azure: `servicePrincipalSecret`, `servicePrincipalCertificate`, `managedIdentity` or `storageAccountKey`. |
| `storage_account_name` | string | ✅ yes | Azure storage account holding the container. |
| `container_name` | string | ✅ yes | Blob container the backups are written to. |
| `service_url` | string | ❌ no | Blob service endpoint, for a sovereign cloud or an emulator. |
| `storage_account_key` | string | ❌ no | Storage account key. Used with `storageAccountKey`. Sensitive. |
| `tenant_id` | string | ❌ no | Entra ID tenant of the service principal. |
| `client_id` | string | ❌ no | Application (client) ID of the service principal. |
| `client_secret` | string | ❌ no | Client secret. Used with `servicePrincipalSecret`. Sensitive. |
| `client_certificate` | string | ❌ no | PEM client certificate. Used with `servicePrincipalCertificate`. Sensitive. |
| `client_certificate_password` | string | ❌ no | Password protecting `client_certificate`. Sensitive. |
| `managed_identity_client_id` | string | ❌ no | Client ID of a user-assigned managed identity; unset means system-assigned. |
| `password` | string | ❌ no | Password the archive itself is encrypted with. Sensitive. |
| `cron_rule` | string | ❌ no | Cron expression for the schedule. Leave unset to keep the schedule off. |

## Attributes Reference

| Name | Type | Description |
|------|------|-------------|
| `last_run_failed` | bool | Whether the most recent scheduled run failed. |
| `last_run_timestamp` | string | UTC timestamp of the most recent scheduled run. |

Credentials are **not** read back from Portainer into state, even though the API returns them: state keeps what the configuration set. Removing the resource stops managing the settings — Portainer has no endpoint to clear them, so the schedule stays as last configured.

Validate the credentials first with the `portainer_backup_azure_connection` data source.
