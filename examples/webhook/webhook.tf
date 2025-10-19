resource "portainer_webhook" "stack" {
  endpoint_id  = var.endpoint_id
  resource_id  = var.resource_id
  webhook_type = var.webhook_type
}
