# Bootstrap only: a restore replaces the instance database.
resource "portainer_backup_azure_restore" "recover" {
  auth_method          = "storageAccountKey"
  storage_account_name = "acmebackups"
  container_name       = "portainer"
  blob_name            = "portainer-2026-09-15.tar.gz"
  storage_account_key  = var.secret
  password             = var.secret
}
