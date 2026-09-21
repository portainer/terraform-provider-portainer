# Resource Documentation: `portainer_backup_azure_execute`

# portainer_backup_azure_execute

> **Business Edition only.** This endpoint does not exist in Portainer CE.


Runs an Azure Blob backup once, outside the schedule.

## Example Usage

```hcl
resource "portainer_backup_azure_execute" "adhoc" {
  auth_method          = "storageAccountKey"
  storage_account_name = "acmebackups"
  container_name       = "portainer"
  storage_account_key  = var.azure_storage_key

  triggers = {
    before_upgrade = var.portainer_version
  }
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
| `triggers` | map | ❌ no | Arbitrary values that force another run when they change. |

## Attributes Reference

| Name | Type | Description |
|------|------|-------------|
| `id` | string | Synthetic identifier carrying a timestamp. |

This does **not** touch the stored schedule: no cron rule is sent. Every argument forces a new resource, since a backup is a one-shot action.
