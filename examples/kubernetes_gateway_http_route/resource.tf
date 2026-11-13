# Requires Portainer Business Edition 3.0 or later.
resource "portainer_kubernetes_gateway_http_route" "app" {
  environment_id = var.environment_id
  namespace      = "default"
  name           = "app"
  hostnames      = ["app.example.com"]

  parent_ref {
    name         = var.gateway_name
    section_name = "http"
  }

  rule {
    match {
      path_type  = "PathPrefix"
      path_value = "/"
    }

    backend_ref {
      name = var.service_name
      port = 80
    }
  }
}
