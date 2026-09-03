data "portainer_registry_connection" "harbor" {
  url           = "registry.example.com"
  type          = 3
  username      = "robot$ci"
  password      = var.secret
  tls           = true
  fail_on_error = true
}

output "harbor_reachable" {
  value = data.portainer_registry_connection.harbor.success
}
