data "portainer_edge_mtls_ca_certificate" "current" {}

output "edge_mtls_ca_configured" {
  value = data.portainer_edge_mtls_ca_certificate.current.configured
}

output "edge_mtls_ca_expiry" {
  value = data.portainer_edge_mtls_ca_certificate.current.valid_not_after
}
