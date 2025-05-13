resource "portainer_stack" "swarm_string" {
  name            = var.stack_name
  deployment_type = var.stack_deployment_type
  method          = var.stack_method
  endpoint_id     = var.stack_endpoint_id

  stack_file_content = var.stack_file_content

  env {
    name  = var.stack_env_name
    value = var.stack_env_value
  }
}
