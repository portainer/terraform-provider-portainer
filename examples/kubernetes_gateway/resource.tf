# Requires Portainer Business Edition 3.0 or later and a Gateway API
# implementation in the cluster.
data "portainer_kubernetes_gateway_classes" "all" {
  environment_id = var.environment_id
}

resource "portainer_kubernetes_gateway" "edge" {
  environment_id = var.environment_id
  namespace      = "default"
  name           = "edge"
  class_name     = data.portainer_kubernetes_gateway_classes.all.gateway_classes[0].name

  listener {
    name     = "http"
    port     = 80
    protocol = "HTTP"
  }
}

output "gateway_resource_version" {
  value = portainer_kubernetes_gateway.edge.resource_version
}
