resource "portainer_backup_azure_settings" "nightly" {
  auth_method          = "servicePrincipalSecret"
  storage_account_name = "acmebackups"
  container_name       = "portainer"

  tenant_id     = "00000000-0000-0000-0000-000000000000"
  client_id     = "11111111-1111-1111-1111-111111111111"
  client_secret = var.secret

  cron_rule = "0 2 * * *"
  password  = var.secret
}

output "last_azure_backup" {
  value = portainer_backup_azure_settings.nightly.last_run_timestamp
}
