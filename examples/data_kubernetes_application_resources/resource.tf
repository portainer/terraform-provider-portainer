data "portainer_kubernetes_application_resources" "prod" {
  environment_id = var.environment_id
}

output "cpu_requested_cores" {
  value = data.portainer_kubernetes_application_resources.prod.cpu_request
}
