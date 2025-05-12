resource "portainer_edge_update_schedules" "example" {
  name           = var.edge_schedule_name
  agent_image    = var.agent_image
  updater_image  = var.updater_image
  registry_id    = var.registry_id
  scheduled_time = var.scheduled_time
  group_ids      = var.edge_group_ids
  type           = var.update_type
}
