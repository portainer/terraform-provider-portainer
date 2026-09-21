data "portainer_stack_conversion" "web" {
  stack_id      = 1
  target_format = "kubernetes"
  namespace     = "apps"
}

output "stack_conversion_files" {
  value = keys(data.portainer_stack_conversion.web.files)
}
