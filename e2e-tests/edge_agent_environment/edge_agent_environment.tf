resource "portainer_settings" "edge_compute" {
  enable_edge_compute_features = true
}

resource "portainer_tag" "edge_tag" {
  name = "edge-test-tag"
}

resource "portainer_environment" "edge_agent" {
  depends_on = [portainer_settings.edge_compute]

  name                   = var.edge_agent_name
  environment_address    = var.edge_agent_address
  type                   = 4 # Edge Agent
  tls_enabled            = true
  tls_skip_verify        = true
  tls_skip_client_verify = true
  tag_ids                = [portainer_tag.edge_tag.id]
}

output "edge_key" {
  value     = portainer_environment.edge_agent.edge_key
  sensitive = true
}

output "edge_id" {
  value = portainer_environment.edge_agent.edge_id
}
