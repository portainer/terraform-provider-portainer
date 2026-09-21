# Data Source Documentation: `portainer_ldap_login_test`

# portainer_ldap_login_test

> **Business Edition only.** This endpoint does not exist in Portainer CE.


Tries a set of credentials against an LDAP directory and reports whether the directory accepted them.

## Example Usage

```hcl
data "portainer_ldap_login_test" "alice" {
  url       = "ldap.example.com:389"
  reader_dn = "cn=readonly,dc=example,dc=com"
  password  = var.ldap_password

  search {
    base_dn             = "ou=people,dc=example,dc=com"
    user_name_attribute = "uid"
  }

  username      = "alice"
  test_password = var.alice_password
}

output "alice_can_log_in" {
  value = data.portainer_ldap_login_test.alice.valid
}
```

## Arguments Reference

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `anonymous_mode` | bool | ❌ no | Whether to bind anonymously instead of using `reader_dn` and `password`. |
| `fail_on_error` | bool | ❌ no | Whether credentials the directory rejects fail the plan. Defaults to `false`, because a rejected login is usually the answer being asked for rather than an error. |
| `password` | string | ❌ no | Password of the reader account. Stored in state as a sensitive value. |
| `reader_dn` | string | ❌ no | Distinguished name of the account used to browse the directory. Leave unset when `anonymous_mode` is enabled. |
| `search` | list(object) | ❌ no | Where and how to look for users. Repeatable, and searched in order. |
| `start_tls` | bool | ❌ no | Whether to upgrade the connection with StartTLS. |
| `test_password` | string | ✅ yes | Password to try for `username`. This is the password being tested, not the reader account's. |
| `tls_skip_verify` | bool | ❌ no | Whether to skip verification of the LDAP server's TLS certificate. |
| `url` | string | ✅ yes | URL of the LDAP server, as `host:port` (for example `ldap.example.com:389`). |
| `username` | string | ✅ yes | Login name to try. |

## Attributes Reference

| Name | Type | Description |
|------|------|-------------|
| `valid` | bool | Whether the directory accepted the credentials. |

### `search`

| Name | Type | Description |
|------|------|-------------|

`password` is the reader account's; `test_password` is the one being tested. They are deliberately separate arguments rather than one reused field, because conflating them would be a security trap.

`fail_on_error` defaults to `false`: a rejected login is usually the answer being asked for rather than an error. Set it to `true` to make a rejection stop the plan.
