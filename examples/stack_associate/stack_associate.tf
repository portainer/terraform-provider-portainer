resource "portainer_stack_associate" "example" {
  endpoint_id      = var.stack_associate_endpoint_id
  stack_id         = var.stack_associate_stack_id
  swarm_id         = var.stack_associate_swarm_id
  orphaned_running = var.stack_associate_orphaned_running
}
