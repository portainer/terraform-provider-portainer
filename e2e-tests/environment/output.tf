output "edge_key" {
  value     = portainer_environment.your-host.edge_key
  sensitive = true
}

output "edge_id" {
  value = portainer_environment.your-host.edge_id
}