# Data Source Documentation: `portainer_ldap_groups`

# portainer_ldap_groups

> **Business Edition only.** This endpoint does not exist in Portainer CE.


Searches an LDAP directory for groups.

## Example Usage

```hcl
data "portainer_ldap_groups" "teams" {
  url       = "ldap.example.com:389"
  reader_dn = "cn=readonly,dc=example,dc=com"
  password  = var.ldap_password

  group_search {
    group_base_dn   = "ou=groups,dc=example,dc=com"
    group_filter    = "(objectClass=groupOfNames)"
    group_attribute = "member"
  }
}
```

## Arguments Reference

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `anonymous_mode` | bool | ❌ no | Whether to bind anonymously instead of using `reader_dn` and `password`. |
| `group_search` | list(object) | ❌ no | Where and how to look for groups. Repeatable, and searched in order. |
| `password` | string | ❌ no | Password of the reader account. Stored in state as a sensitive value. |
| `reader_dn` | string | ❌ no | Distinguished name of the account used to browse the directory. Leave unset when `anonymous_mode` is enabled. |
| `start_tls` | bool | ❌ no | Whether to upgrade the connection with StartTLS. |
| `tls_skip_verify` | bool | ❌ no | Whether to skip verification of the LDAP server's TLS certificate. |
| `url` | string | ✅ yes | URL of the LDAP server, as `host:port` (for example `ldap.example.com:389`). |

## Attributes Reference

| Name | Type | Description |
|------|------|-------------|
| `entries` | list(object) | The groups the directory returned for the searches above. |

### `entries`

| Name | Type | Description |
|------|------|-------------|
| `groups` | list(string) | Groups the entry belongs to. |
| `name` | string | Name the directory reports for the entry. |

### `group_search`

| Name | Type | Description |
|------|------|-------------|

These endpoints take the whole directory configuration with the request rather than reusing the instance's saved settings, which is what lets a configuration be tried out before it is committed with `portainer_ldap_settings`. `reader_dn` and `password` are omitted from the request when unset, so `anonymous_mode` really does bind anonymously rather than with a blank password.
