data "portainer_kubernetes_gateway_classes" "all" {
  environment_id = var.k8s_environment_id
}

resource "portainer_kubernetes_gateway" "e2e" {
  environment_id = var.k8s_environment_id
  namespace      = "default"
  name           = "e2e"
  class_name     = data.portainer_kubernetes_gateway_classes.all.gateway_classes[0].name

  listener {
    name     = "http"
    port     = var.port
    protocol = "HTTP"
  }
}

resource "portainer_kubernetes_gateway_http_route" "e2e" {
  environment_id = var.k8s_environment_id
  namespace      = "default"
  name           = "e2e"
  hostnames      = ["app.example.com"]

  parent_ref {
    name         = portainer_kubernetes_gateway.e2e.name
    section_name = "http"
  }

  rule {
    match {
      path_type  = "PathPrefix"
      path_value = "/"
    }

    backend_ref {
      name = "kubernetes"
      port = 443
    }
  }
}
