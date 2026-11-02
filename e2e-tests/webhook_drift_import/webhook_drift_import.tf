resource "portainer_webhook" "drift" {
  endpoint_id  = var.endpoint_id
  resource_id  = "e2e-webhook-drift-service"
  webhook_type = 1
}
