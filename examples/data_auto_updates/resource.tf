data "portainer_auto_updates" "history" {}

output "auto_update_runs" {
  value = data.portainer_auto_updates.history.auto_updates
}

output "auto_update_in_progress" {
  value = length([
    for u in data.portainer_auto_updates.history.auto_updates : u
    if u.status == "inProgress"
  ]) > 0
}
