# Container webhooks (type 2) exist only in Business Edition; the container is
# started by e2e-tests/lib/be-run.sh.
resource "portainer_webhook" "be" {
  endpoint_id  = var.endpoint_id
  resource_id  = var.container_id
  webhook_type = 2
}
