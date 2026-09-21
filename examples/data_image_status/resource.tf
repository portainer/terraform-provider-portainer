data "portainer_image_status" "stack" {
  kind        = "stack"
  resource_id = "1"
}

output "stack_image_status" {
  value = data.portainer_image_status.stack.status
}
