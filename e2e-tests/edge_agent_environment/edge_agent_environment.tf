resource "portainer_tag" "edge_tag" {
  name = "edge-test-tag"
}

resource "portainer_environment" "edge_agent" {
  name                       = var.edge_agent_name
  environment_address        = var.edge_agent_address
  type                       = 4 # Edge Agent
  edge_tunnel_server_address = var.edge_tunnel_server_address
  edge_checkin_interval      = 5
  tls_enabled                = true
  tls_skip_verify            = true
  tls_skip_client_verify     = true
  tag_ids                    = [portainer_tag.edge_tag.id]
}

output "edge_key" {
  value     = portainer_environment.edge_agent.edge_key
  sensitive = true
}

output "edge_id" {
  value = portainer_environment.edge_agent.edge_id
}
