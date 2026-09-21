data "portainer_kubernetes_application" "web" {
  endpoint_id = 1
  namespace   = "default"
  name        = "web"
}

output "kubernetes_application_pods_missing" {
  value = data.portainer_kubernetes_application.web.total_pods - data.portainer_kubernetes_application.web.running_pods
}
