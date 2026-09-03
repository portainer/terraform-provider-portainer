resource "portainer_kubernetes_application" "scale_target" {
  endpoint_id = var.endpoint_id
  namespace   = var.namespace
  manifest    = file("${path.module}/application.yaml")
}

resource "portainer_kubernetes_deployment_scale" "test" {
  environment_id  = var.endpoint_id
  namespace       = var.namespace
  deployment_name = "nginx-scale-test"
  replicas        = 2

  depends_on = [portainer_kubernetes_application.scale_target]
}

output "scaled_replicas" {
  value = portainer_kubernetes_deployment_scale.test.replicas

  precondition {
    condition     = portainer_kubernetes_deployment_scale.test.replicas == 2
    error_message = "The deployment must report the replica count it was scaled to."
  }
}
