resource "portainer_webhook_execute" "test_token" {
  token = var.webhook_token
}

# nebo pro Git stack update
# resource "portainer_webhook_execute" "test_stack" {
#   stack_id = var.stack_id
# }