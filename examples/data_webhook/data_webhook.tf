data "portainer_webhook" "example" {
  resource_id = var.resource_id
  endpoint_id = var.endpoint_id
}

output "webhook_id" {
  value = data.portainer_webhook.example.id
}

output "webhook_type" {
  value = data.portainer_webhook.example.webhook_type
}

output "webhook_token" {
  value     = data.portainer_webhook.example.token
  sensitive = true
}
