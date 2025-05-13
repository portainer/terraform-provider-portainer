resource "portainer_stack" "standalone_string" {
  name            = var.stack_name
  deployment_type = var.stack_deployment_type
  method          = var.stack_method
  endpoint_id     = var.stack_endpoint_id

  repository_url            = var.stack_repository_url
  file_path_in_repository   = var.stack_file_path_in_repository
  repository_reference_name = var.stack_repository_reference_name

  env {
    name  = var.stack_env_name
    value = var.stack_env_value
  }
}
