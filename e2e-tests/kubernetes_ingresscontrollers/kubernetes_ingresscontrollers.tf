resource "portainer_kubernetes_ingresscontrollers" "test" {
  environment_id = var.environment_id

  dynamic "controllers" {
    for_each = var.controllers
    content {
      name         = controllers.value.name
      class_name   = controllers.value.class_name
      type         = controllers.value.type
      availability = controllers.value.availability
      used         = controllers.value.used
      new          = controllers.value.new
    }
  }
}
