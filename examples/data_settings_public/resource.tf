data "portainer_settings_public" "this" {}

output "authentication_method" {
  value = data.portainer_settings_public.this.authentication_method
}

output "edge_compute_enabled" {
  value = data.portainer_settings_public.this.enable_edge_compute_features
}
