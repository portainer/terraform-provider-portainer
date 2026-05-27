output "webhook_token" {
  value     = portainer_webhook.stack.token
  sensitive = true
}
