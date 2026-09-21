# Data Source Documentation: `portainer_ldap_users`

# portainer_ldap_users

> **Business Edition only.** This endpoint does not exist in Portainer CE.


Searches an LDAP directory for users, using a directory configuration supplied here rather than the one Portainer has saved.

## Example Usage

```hcl
data "portainer_ldap_users" "people" {
  url       = "ldap.example.com:389"
  reader_dn = "cn=readonly,dc=example,dc=com"
  password  = var.ldap_password

  search {
    base_dn             = "ou=people,dc=example,dc=com"
    filter              = "(objectClass=person)"
    user_name_attribute = "uid"
  }
}

output "ldap_user_names" {
  value = [for u in data.portainer_ldap_users.people.entries : u.name]
}
```

## Arguments Reference

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `anonymous_mode` | bool | ❌ no | Whether to bind anonymously instead of using `reader_dn` and `password`. |
| `password` | string | ❌ no | Password of the reader account. Stored in state as a sensitive value. |
| `reader_dn` | string | ❌ no | Distinguished name of the account used to browse the directory. Leave unset when `anonymous_mode` is enabled. |
| `search` | list(object) | ❌ no | Where and how to look for users. Repeatable, and searched in order. |
| `start_tls` | bool | ❌ no | Whether to upgrade the connection with StartTLS. |
| `tls_skip_verify` | bool | ❌ no | Whether to skip verification of the LDAP server's TLS certificate. |
| `url` | string | ✅ yes | URL of the LDAP server, as `host:port` (for example `ldap.example.com:389`). |

## Attributes Reference

| Name | Type | Description |
|------|------|-------------|
| `entries` | list(object) | The users the directory returned for the searches above. |

### `entries`

| Name | Type | Description |
|------|------|-------------|
| `groups` | list(string) | Groups the entry belongs to. |
| `name` | string | Name the directory reports for the entry. |

### `search`

| Name | Type | Description |
|------|------|-------------|

These endpoints take the whole directory configuration with the request rather than reusing the instance's saved settings, which is what lets a configuration be tried out before it is committed with `portainer_ldap_settings`. `reader_dn` and `password` are omitted from the request when unset, so `anonymous_mode` really does bind anonymously rather than with a blank password.
