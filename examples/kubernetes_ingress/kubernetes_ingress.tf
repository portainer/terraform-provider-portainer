resource "portainer_kubernetes_ingresses" "example" {
  environment_id = var.environment_id
  namespace      = var.namespace
  name           = var.ingress_name
  class_name     = var.class_name

  annotations = var.annotations
  labels      = var.labels
  hosts       = var.hosts

  tls {
    hosts       = var.tls_hosts
    secret_name = var.tls_secret_name
  }

  paths {
    host         = var.path_host
    path         = var.path
    path_type    = var.path_type
    port         = var.service_port
    service_name = var.service_name
  }
}
