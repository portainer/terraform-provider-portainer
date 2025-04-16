resource "portainer_kubernetes_namespace_ingresscontrollers" "test" {
  environment_id = var.environment_id
  namespace      = var.namespace

  controllers {
    name         = var.ingress_controller.name
    class_name   = var.ingress_controller.class_name
    type         = var.ingress_controller.type
    availability = var.ingress_controller.availability
    used         = var.ingress_controller.used
    new          = var.ingress_controller.new
  }
}
