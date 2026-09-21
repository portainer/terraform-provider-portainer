# Bootstrap only: Portainer accepts a restore on an instance that has not been
# initialised yet. The archive and its password both land in Terraform state.
resource "portainer_restore" "bootstrap" {
  file_content_base64 = var.backup_archive_base64
  file_name           = "portainer-backup.tar.gz"
  password            = var.secret
  setup_token         = var.secret
}
