resource "portainer_webhook" "example" {
  endpoint_id  = var.endpoint_id
  resource_id  = var.resource_id
  webhook_type = var.webhook_type
}
