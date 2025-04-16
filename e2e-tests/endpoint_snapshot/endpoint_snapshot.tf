resource "portainer_endpoint_snapshot" "example" {
  endpoint_id = var.endpoint_id # Set to null or remove to snapshot all
}
