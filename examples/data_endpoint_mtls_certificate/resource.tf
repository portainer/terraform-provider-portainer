data "portainer_endpoint_mtls_certificate" "agent" {
  endpoint_id = 12
}

output "endpoint_mtls_certificate_configured" {
  value = data.portainer_endpoint_mtls_certificate.agent.configured
}

output "endpoint_mtls_certificate_expiry" {
  value = data.portainer_endpoint_mtls_certificate.agent.valid_not_after
}
