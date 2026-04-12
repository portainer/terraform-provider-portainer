resource "portainer_stack_migrate" "test" {
  stack_id           = var.stack_id
  target_endpoint_id = var.target_endpoint_id
}
