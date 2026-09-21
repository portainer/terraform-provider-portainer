resource "portainer_backup_local_run" "before_upgrade" {
  triggers = {
    version = "2.45.0"
  }
}

output "backup_path" {
  value = portainer_backup_local_run.before_upgrade.path
}
