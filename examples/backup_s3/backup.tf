resource "portainer_backup_s3" "your_s3_backup" {
  access_key_id      = var.s3_access_key
  secret_access_key  = var.s3_secret_key
  bucket_name        = var.s3_bucket
  region             = var.s3_region
  s3_compatible_host = var.s3_endpoint
  password           = var.backup_password
  cron_rule          = var.backup_cron_rule
}
