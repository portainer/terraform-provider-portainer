data "portainer_observability_environment" "this" {
  environment_id = var.environment_id
}

output "observability_enabled" {
  value = data.portainer_observability_environment.this.enabled
}

output "observability_url" {
  value = data.portainer_observability_environment.this.url
}
