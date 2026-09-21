resource "portainer_settings" "edge_compute" {
  enable_edge_compute_features = true
}

resource "portainer_tag" "edge_tag" {
  name = "edge-test-tag"
}

# No tls_* attributes here on purpose: Portainer rejects TLS for Edge Agent
# environments, and the provider drops those fields for edge types anyway. The
# agent reaches Portainer through the edge tunnel and presents no certificate.
resource "portainer_environment" "edge_agent" {
  depends_on = [portainer_settings.edge_compute]

  name                = var.edge_agent_name
  environment_address = var.edge_agent_address
  type                = 4 # Edge Agent
  tag_ids             = [portainer_tag.edge_tag.id]
}
