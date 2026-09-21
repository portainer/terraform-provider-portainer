# Data Source Documentation: `portainer_edge_mtls_certificate`

# portainer_edge_mtls_certificate

> **Business Edition only.** This endpoint does not exist in Portainer CE.


Reads the parsed X.509 details of the certificate Portainer presents to edge agents.

## Example Usage

```hcl
data "portainer_edge_mtls_certificate" "current" {}

output "edge_mtls_certificate_expiry" {
  value = data.portainer_edge_mtls_certificate.current.valid_not_after
}
```

## Attributes Reference

| Name | Type | Description |
|------|------|-------------|
| `configured` | bool | Whether Portainer has this certificate configured. Every other attribute is empty when it does not. |
| `common_name` | string | Common name of the certificate's subject. |
| `organization` | list(string) | Organization of the certificate's subject. |
| `issuer_common_name` | string | Common name of the certificate's issuer. |
| `serial_number` | string | Serial number of the certificate. |
| `sha256_fingerprint` | string | SHA-256 fingerprint, which is what to compare against when rotating. |
| `signature_algorithm` | string | Algorithm the certificate is signed with. |
| `is_certificate_authority` | bool | Whether the certificate carries the CA basic constraint. |
| `valid_not_before` | string | Start of the validity period. |
| `valid_not_after` | string | End of the validity period. |
| `version` | number | X.509 version of the certificate. |
| `subject_alt_dns_names` | list(string) | DNS names in the subject alternative name extension. |
| `subject_alt_ip_addresses` | list(string) | IP addresses in the subject alternative name extension. |
| `key_usages` | list(string) | Key usages the certificate permits. |
| `extended_key_usages` | list(string) | Extended key usages the certificate permits. |
| `public_key_algorithm` | string | Algorithm of the public key. |
| `public_key_size` | number | Size of the public key, in bits. |

This data source is read-only. Portainer takes its edge mTLS material from files on the host, so there is no endpoint to write it through - use `valid_not_after` to drive an alert or a rotation workflow before edge agents stop connecting.
