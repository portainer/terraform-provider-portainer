# Data Source Documentation: `portainer_ldap_check`

# portainer_ldap_check
The `portainer_ldap_check` data source asks Portainer to bind to an LDAP directory with the given settings, as a pre-flight check before those settings are committed with `portainer_ldap_settings`.

Unlike the other connection checks in this provider, `fail_on_error` defaults to **true** here: a check that silently reports a broken directory would let a broken authentication configuration be applied in the same plan.

## Example Usage

```hcl
data "portainer_ldap_check" "corp" {
  url       = "ldap.example.com:389"
  reader_dn = "cn=reader,dc=example,dc=com"
  password  = var.ldap_password
  start_tls = true
}

resource "portainer_ldap_settings" "corp" {
  depends_on = [data.portainer_ldap_check.corp]
  # ...
}
```

## Arguments Reference

| Name              | Type   | Required | Default | Description                                                                             |
|-------------------|--------|----------|---------|-------------------------------------------------------------------------------------------|
| `url`             | string | ✅ yes   | –       | URL of the LDAP server as `host:port`.                                                    |
| `reader_dn`       | string | ❌ no    | –       | Distinguished name of the account used to browse the directory.                            |
| `password`        | string | ❌ no    | –       | Password of the reader account. Stored in state as a sensitive value.                      |
| `anonymous_mode`  | bool   | ❌ no    | `false` | Bind anonymously instead of using `reader_dn` and `password`.                               |
| `start_tls`       | bool   | ❌ no    | `false` | Upgrade the connection with StartTLS.                                                       |
| `tls_skip_verify` | bool   | ❌ no    | `false` | Skip verification of the server's TLS certificate.                                          |
| `fail_on_error`   | bool   | ❌ no    | `true`  | Whether a failed bind makes the data source itself fail.                                    |

## Attributes Reference

| Name      | Type   | Description                                                    |
|-----------|--------|----------------------------------------------------------------|
| `success` | bool   | Whether Portainer could reach and bind to the directory.       |
| `error`   | string | Reason the check failed, empty on success.                     |

The endpoint answers `204` on success and `400`/`500` with a message otherwise, so the outcome is the status code rather than a response body.
