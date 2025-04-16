resource "portainer_kubernetes_helm" "example" {
  environment_id = var.environment_id
  chart          = var.helm_chart
  name           = var.helm_release_name
  namespace      = var.helm_namespace
  repo           = var.helm_repo
  values         = var.helm_values
}
