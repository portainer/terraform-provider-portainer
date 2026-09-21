# Bootstrap only: a restore replaces the instance database.
resource "portainer_backup_s3_restore" "recover" {
  bucket_name       = "acme-backups"
  filename          = "portainer-2026-09-15.tar.gz"
  region            = "eu-central-1"
  access_key_id     = "AKIAEXAMPLE"
  secret_access_key = var.secret
  password          = var.secret
}
