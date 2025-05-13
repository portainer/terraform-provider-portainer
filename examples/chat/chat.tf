resource "portainer_chat" "test" {
  context        = var.chat_context
  environment_id = var.chat_environment_id
  message        = var.chat_message
  model          = var.chat_model
}
