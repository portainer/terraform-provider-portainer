data "portainer_edge_mtls_certificate" "current" {}

output "edge_mtls_certificate_expiry" {
  value = data.portainer_edge_mtls_certificate.current.valid_not_after
}

output "edge_mtls_certificate_fingerprint" {
  value = data.portainer_edge_mtls_certificate.current.sha256_fingerprint
}
