data "portainer_kubernetes_replicasets" "web" {
  environment_id = var.environment_id
  namespace      = "default"
  deployment     = "web"
  label_selector = "app=web"
  field_selector = ""
}

output "web_revisions" {
  value = {
    for rs in data.portainer_kubernetes_replicasets.web.replicasets : rs.name => rs.revision
  }
}
