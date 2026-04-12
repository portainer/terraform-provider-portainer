# Resource Documentation: `portainer_ldap_settings`

# portainer_ldap_settings
The `portainer_ldap_settings` resource manages the LDAP authentication configuration in Portainer. It sets the authentication method to LDAP and configures all LDAP-related settings including server URLs, search settings, group mappings, and TLS.

## Example Usage

```hcl
resource "portainer_ldap_settings" "ldap" {
  url       = "ldap://ldap.example.com:389"
  reader_dn = "cn=readonly,dc=example,dc=com"
  password  = "ldap-readonly-password"

  search_settings {
    base_dn             = "dc=example,dc=com"
    filter              = "(objectClass=person)"
    user_name_attribute = "uid"
  }

  group_search_settings {
    group_base_dn   = "ou=groups,dc=example,dc=com"
    group_filter    = "(objectClass=groupOfNames)"
    group_attribute = "member"
  }

  tls_config {
    tls             = true
    tls_skip_verify = false
  }
}
```
- [Example on GitHub](https://github.com/portainer/terraform-provider-portainer/tree/main/examples/ldap_settings)

## Lifecycle & Behavior
This resource updates the Portainer global settings to configure LDAP authentication. Deleting this resource only removes it from Terraform state; it does not revert LDAP settings.

## Arguments Reference

| Name                         | Type         | Required | Description                                                        |
|------------------------------|--------------|----------|--------------------------------------------------------------------|
| `anonymous_mode`             | bool         | no       | Enable anonymous mode (ReaderDN and Password not used).            |
| `auto_create_users`          | bool         | no       | Automatically provision users from LDAP.                           |
| `password`                   | string       | no       | Password for the search account (sensitive).                       |
| `reader_dn`                  | string       | no       | DN of the account used to search users.                            |
| `start_tls`                  | bool         | no       | Whether to use StartTLS.                                           |
| `url`                        | string       | no       | LDAP server URL (deprecated, use `urls`).                          |
| `urls`                       | list(string) | no       | List of LDAP server URLs.                                          |
| `server_type`                | int          | no       | LDAP server type.                                                  |
| `admin_auto_populate`        | bool         | no       | Enable auto admin population.                                      |
| `admin_groups`               | list(string) | no       | Admin group list for role mapping.                                 |
| `search_settings`            | list(object) | no       | LDAP user search settings (base_dn, filter, user_name_attribute).  |
| `group_search_settings`      | list(object) | no       | LDAP group search settings (group_attribute, group_base_dn, group_filter). |
| `admin_group_search_settings`| list(object) | no       | LDAP admin group search settings.                                  |
| `tls_config`                 | list(object) | no       | TLS configuration (tls, tls_ca_cert, tls_cert, tls_key, tls_skip_verify). |
