data "portainer_edge_waiting_room" "pending" {}

output "edge_environments_awaiting_trust" {
  value = data.portainer_edge_waiting_room.pending.endpoint_ids
}

output "edge_waiting_room_details" {
  value = data.portainer_edge_waiting_room.pending.environments
}
