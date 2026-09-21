# Data Source Documentation: `portainer_endpoint_mtls_certificate`

# portainer_endpoint_mtls_certificate

> **Business Edition only.** This endpoint does not exist in Portainer CE.


Reads the parsed X.509 details of the mTLS certificate an environment's agent is connecting with.

## Example Usage

```hcl
data "portainer_endpoint_mtls_certificate" "agent" {
  endpoint_id = portainer_endpoint_trust.shop_floor_3.endpoint_id
}

output "active_certificate_expiry" {
  value = data.portainer_endpoint_mtls_certificate.agent.valid_not_after
}
```

## Arguments Reference

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `endpoint_id` | number | ✅ yes | Identifier of the environment whose certificate is read. |

## Attributes Reference

| Name | Type | Description |
|------|------|-------------|
| `configured` | bool | Whether Portainer holds such a certificate for the environment. Every other attribute is empty when it does not. |
| `common_name` | string | Common name of the certificate's subject. |
| `organization` | list(string) | Organization of the certificate's subject. |
| `issuer_common_name` | string | Common name of the certificate's issuer, which is the edge mTLS CA. |
| `serial_number` | string | Serial number of the certificate. |
| `sha256_fingerprint` | string | SHA-256 fingerprint of the certificate. |
| `signature_algorithm` | string | Algorithm the certificate is signed with. |
| `is_certificate_authority` | bool | Whether the certificate carries the CA basic constraint. An agent certificate is a leaf, so this reports `false`. |
| `valid_not_before` | string | Start of the validity period. |
| `valid_not_after` | string | End of the validity period. |
| `version` | number | X.509 version of the certificate. |
| `subject_alt_dns_names` | list(string) | DNS names in the subject alternative name extension. |
| `subject_alt_ip_addresses` | list(string) | IP addresses in the subject alternative name extension. |
| `key_usages` | list(string) | Key usages the certificate permits. |
| `extended_key_usages` | list(string) | Extended key usages the certificate permits. |
| `public_key_algorithm` | string | Algorithm of the public key. |
| `public_key_size` | number | Size of the public key, in bits. |

An environment that has never presented a certificate reports `configured = false` with every other attribute empty, rather than failing the plan. A call that fails for any other reason still fails.
