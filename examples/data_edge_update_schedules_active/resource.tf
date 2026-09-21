data "portainer_edge_update_schedules_active" "shops" {
  environment_ids = [1]
}

output "edge_updates_in_flight" {
  value = length(data.portainer_edge_update_schedules_active.shops.schedules) > 0
}
