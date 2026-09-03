# Resource Documentation: `portainer_registry_configure`

# portainer_registry_configure
The `portainer_registry_configure` resource applies the authentication and TLS configuration of an existing registry, which is a separate Portainer endpoint from the registry object itself.

## Example Usage

```hcl
resource "portainer_registry" "harbor" {
  name = "harbor"
  url  = "registry.example.com"
  type = 3
}

resource "portainer_registry_configure" "harbor" {
  registry_id    = portainer_registry.harbor.id
  authentication = true
  username       = "robot$ci"
  password       = var.registry_password
  tls            = true
  tls_ca_cert    = file("ca.pem")
}
```

## Arguments Reference

| Name              | Type   | Required | Default | Description                                                                                     |
|-------------------|--------|----------|---------|-------------------------------------------------------------------------------------------------|
| `registry_id`     | number | ✅ yes   | –       | Identifier of the registry to configure. Changing it replaces the resource.                        |
| `authentication`  | bool   | ✅ yes   | –       | Whether the registry requires authentication. When false, Portainer ignores the credentials.        |
| `username`        | string | ❌ no    | –       | Username used to authenticate. Required when `authentication` is true.                              |
| `password`        | string | ❌ no    | –       | Password or token paired with `username`. Stored in state as a sensitive value.                     |
| `region`          | string | ❌ no    | –       | AWS region of an ECR registry. Ignored by other types.                                              |
| `tls`             | bool   | ❌ no    | `false` | Whether Portainer contacts the registry over TLS.                                                   |
| `tls_skip_verify` | bool   | ❌ no    | `false` | Skip verification of the registry's TLS certificate.                                                |
| `tls_ca_cert`     | string | ❌ no    | –       | PEM-encoded CA certificate. Stored in state as a sensitive value.                                    |
| `tls_cert`        | string | ❌ no    | –       | PEM-encoded client certificate. Stored in state as a sensitive value.                                |
| `tls_key`         | string | ❌ no    | –       | PEM-encoded client private key. Stored in state as a sensitive value.                                |

## Attributes Reference

| Name | Description                                                    |
|------|----------------------------------------------------------------|
| `id` | Composite identifier in the form `<registry_id>/configure`.     |

Portainer has no endpoint to read a registry's configuration back or to clear it, so this resource cannot detect drift, and destroying it only stops managing the configuration — the registry keeps the settings it was given.
