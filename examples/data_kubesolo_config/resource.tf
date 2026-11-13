data "portainer_kubesolo_config" "edge" {
  environment_id = var.environment_id
}

output "kubesolo_version" {
  value = data.portainer_kubesolo_config.edge.version
}

output "kubesolo_api" {
  value = data.portainer_kubesolo_config.edge.api
}

output "kubesolo_config" {
  value     = data.portainer_kubesolo_config.edge.config_json != "" ? jsondecode(data.portainer_kubesolo_config.edge.config_json) : null
  sensitive = true
}
