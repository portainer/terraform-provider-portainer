data "portainer_user_activity" "recent" {
  log_type = var.log_type
  limit    = var.log_limit
}

output "activity_count" {
  value = length(data.portainer_user_activity.recent.activity_logs)
}

output "total_count" {
  value = data.portainer_user_activity.recent.total_count
}
