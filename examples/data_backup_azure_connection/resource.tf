# fail_on_error defaults to true: a broken backup configuration must not be
# applied in the same plan that "checked" it.
data "portainer_backup_azure_connection" "check" {
  auth_method          = "storageAccountKey"
  storage_account_name = "acmebackups"
  container_name       = "portainer"
  storage_account_key  = var.secret
}

output "azure_container_reachable" {
  value = data.portainer_backup_azure_connection.check.success
}
