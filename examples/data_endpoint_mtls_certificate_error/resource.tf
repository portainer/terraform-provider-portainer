data "portainer_endpoint_mtls_certificate_error" "agent" {
  endpoint_id = 12
}

output "endpoint_mtls_rejected_certificate_cn" {
  value = data.portainer_endpoint_mtls_certificate_error.agent.common_name
}

output "endpoint_mtls_rejected_certificate_issuer" {
  value = data.portainer_endpoint_mtls_certificate_error.agent.issuer_common_name
}
