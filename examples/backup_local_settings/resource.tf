resource "portainer_backup_local_settings" "nightly" {
  cron_rule      = "0 3 * * *"
  retention_days = 14
}

output "last_local_backup_path" {
  value = portainer_backup_local_settings.nightly.last_run_path
}
