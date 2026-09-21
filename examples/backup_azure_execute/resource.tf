# Ad-hoc backup. No cron rule is sent, so the stored schedule is untouched.
resource "portainer_backup_azure_execute" "adhoc" {
  auth_method          = "storageAccountKey"
  storage_account_name = "acmebackups"
  container_name       = "portainer"
  storage_account_key  = var.secret

  triggers = {
    before_upgrade = "2.45.0"
  }
}
