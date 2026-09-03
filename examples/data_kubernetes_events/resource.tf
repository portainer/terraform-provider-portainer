data "portainer_kubernetes_events" "prod" {
  environment_id = var.environment_id
  namespace      = "prod"
}

output "problems" {
  value = [
    for e in data.portainer_kubernetes_events.prod.events : "${e.kind}/${e.name}: ${e.reason} - ${e.message}"
    if e.type == "Warning"
  ]
}
