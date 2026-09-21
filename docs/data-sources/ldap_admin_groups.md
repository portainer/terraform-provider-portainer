# Data Source Documentation: `portainer_ldap_admin_groups`

# portainer_ldap_admin_groups

> **Business Edition only.** This endpoint does not exist in Portainer CE.


Fetches the LDAP groups whose members Portainer would make administrators, for the admin group search settings given here.

## Example Usage

```hcl
data "portainer_ldap_admin_groups" "admins" {
  url       = "ldap.example.com:389"
  reader_dn = "cn=readonly,dc=example,dc=com"
  password  = var.ldap_password

  admin_group_search {
    group_base_dn = "ou=groups,dc=example,dc=com"
    group_filter  = "(cn=*-admins)"
  }
}

output "portainer_admin_groups" {
  value = data.portainer_ldap_admin_groups.admins.group_names
}
```

## Arguments Reference

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `admin_group_search` | list(object) | ❌ no | Where and how to look for the groups whose members become administrators. Repeatable, and searched in order. |
| `anonymous_mode` | bool | ❌ no | Whether to bind anonymously instead of using `reader_dn` and `password`. |
| `password` | string | ❌ no | Password of the reader account. Stored in state as a sensitive value. |
| `reader_dn` | string | ❌ no | Distinguished name of the account used to browse the directory. Leave unset when `anonymous_mode` is enabled. |
| `start_tls` | bool | ❌ no | Whether to upgrade the connection with StartTLS. |
| `tls_skip_verify` | bool | ❌ no | Whether to skip verification of the LDAP server's TLS certificate. |
| `url` | string | ✅ yes | URL of the LDAP server, as `host:port` (for example `ldap.example.com:389`). |

## Attributes Reference

| Name | Type | Description |
|------|------|-------------|
| `group_names` | list(string) | Names of the groups whose members Portainer would make administrators. |

### `admin_group_search`

| Name | Type | Description |
|------|------|-------------|

This endpoint answers with a plain list of names rather than the entry objects the user and group searches return. These endpoints take the whole directory configuration with the request rather than reusing the instance's saved settings, which is what lets a configuration be tried out before it is committed with `portainer_ldap_settings`. `reader_dn` and `password` are omitted from the request when unset, so `anonymous_mode` really does bind anonymously rather than with a blank password.
