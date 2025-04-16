# 🛡️ **Resource Documentation: `portainer_open_amt`**

# portainer_open_amt
The `portainer_open_amt` resource enables and configures Intel® OpenAMT capabilities in Portainer.

## Example Usage
```hcl
resource "portainer_open_amt" "enable" {
  enabled            = true
  domain_name        = "amt.local"
  mpsserver          = "https://amt-proxy.local"
  mpsuser            = "admin"
  mpspassword        = "supersecret"

  cert_file_name     = "cert.pfx"
  cert_file_password = "certpassword"
  cert_file_content  = filebase64("certs/cert.pfx")
}
```

## Lifecycle & Behavior
- This resource enables OpenAMT and uploads the required certificate.
- It is managed globally (not per endpoint).
- Destroying this resource does not disable OpenAMT (API does not provide such an operation).
- Reapplying with different values will overwrite the current OpenAMT config.

## Arguments Reference
| Name                | Type    | Required | Description                                                                |
|---------------------|---------|----------|----------------------------------------------------------------------------|
| `enabled`           | bool    | ✅ yes   | Whether to enable OpenAMT integration                                      |
| `domain_name`       | string  | ✅ yes   | Domain used by OpenAMT                                                     |
| `mpsserver`         | string  | ✅ yes   | URL of the MPS (Management Presence Server), e.g., `https://proxy.local`  |
| `mpsuser`           | string  | ✅ yes   | MPS username                                                               |
| `mpspassword`       | string  | ✅ yes   | MPS password (sensitive)                                                   |
| `cert_file_name`    | string  | ✅ yes   | Name of the certificate file (e.g., `cert.pfx`)                            |
| `cert_file_password`| string  | ✅ yes   | Password for the certificate (sensitive)                                   |
| `cert_file_content` | string  | ✅ yes   | Base64-encoded certificate content (use `filebase64("certs/cert.pfx")`)    |

## Attributes Reference

| Name | Description              |
|------|--------------------------|
| `id` | ID of the created open_amt |
