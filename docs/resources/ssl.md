# 🔐 **Resource Documentation: `portainer_ssl`**

# portainer_ssl

This resource allows you to configure the SSL certificate and key for your Portainer instance.
It supports toggling HTTP access and providing the certificate and key as strings (e.g., using the `file()` function).

> ⚠️ This resource requires administrator access.

## Example Usage
```hcl
resource "portainer_ssl" "cert_update" {
  cert         = file("certs/server.crt")
  key          = file("certs/server.key")
  http_enabled = false
}
```

## Lifecycle & Behavior
SSL of Portainer are modify if any of the arguments/files change by run:
```hcl
terraform apply
```

## Example make SSL ceert
```hcl
$ mkdir certs
$ openssl req -x509 -newkey rsa:4096 -keyout certs/server.key -out certs/server.crt -days 365 -nodes -subj "/CN=localhost"
```

## Arguments Reference
| Name           | Type   | Required | Description                                                              |
|----------------|--------|----------|--------------------------------------------------------------------------|
| `cert`         | string | ✅ yes   | Contents of the SSL certificate (e.g., `file("certs/server.crt")`)      |
| `key`          | string | ✅ yes   | Contents of the private key (e.g., `file("certs/server.key")`)          |
| `client_cert`  | string | 🚫 optional | Contents of the client certificate, if required for mutual TLS          |
| `http_enabled` | bool   | 🚫 optional | Whether to keep HTTP access enabled (default: `false`)                  |

## Attributes Reference

| Name | Description              |
|------|--------------------------|
| `id` | Always `"portainer-settings"` |

## Import

The SSL configuration is a singleton resource; import using the fixed ID `portainer-ssl`:

```shell
terraform import portainer_ssl.example portainer-ssl
```
