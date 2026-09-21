# Data Source Documentation: `portainer_backup_azure_connection`

# portainer_backup_azure_connection

> **Business Edition only.** This endpoint does not exist in Portainer CE.


Asks Portainer to test whether it can reach an Azure Blob container with the given credentials, before they are committed with `portainer_backup_azure_settings`.

## Example Usage

```hcl
data "portainer_backup_azure_connection" "check" {
  auth_method          = "servicePrincipalSecret"
  storage_account_name = "acmebackups"
  container_name       = "portainer"
  tenant_id            = var.azure_tenant_id
  client_id            = var.azure_client_id
  client_secret        = var.azure_client_secret
}

resource "portainer_backup_azure_settings" "nightly" {
  depends_on = [data.portainer_backup_azure_connection.check]
  # ...
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
| `fail_on_error` | bool | ❌ no | Whether an unreachable container fails the data source. Defaults to **true** — a silently reported failure would let a broken backup configuration be applied in the same plan. |

## Attributes Reference

| Name | Type | Description |
|------|------|-------------|
| `success` | bool | Whether Portainer could reach the container. |
| `error` | string | Reason the check failed, empty on success. |

The endpoint answers with a status code rather than a result object, so the outcome is the code itself.
