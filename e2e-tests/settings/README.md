<!-- BEGIN_TF_DOCS -->


## Providers

| Name | Version |
|------|---------|
| <a name="provider_portainer"></a> [portainer](#provider\_portainer) | n/a |

## Resources

| Name | Type |
|------|------|
| [portainer_settings.example](https://registry.terraform.io/providers/portainer/portainer/latest/docs/resources/settings) | resource |

## Inputs

| Name | Description | Type | Default | Required |
|------|-------------|------|---------|:--------:|
| <a name="input_authentication_method"></a> [authentication\_method](#input\_authentication\_method) | Authentication method | `number` | `1` | no |
| <a name="input_enable_telemetry"></a> [enable\_telemetry](#input\_enable\_telemetry) | Enable Portainer telemetry | `bool` | `false` | no |
| <a name="input_ldap_anonymous_mode"></a> [ldap\_anonymous\_mode](#input\_ldap\_anonymous\_mode) | Enable anonymous LDAP mode | `bool` | `true` | no |
| <a name="input_ldap_auto_create_users"></a> [ldap\_auto\_create\_users](#input\_ldap\_auto\_create\_users) | Auto-create users from LDAP | `bool` | `true` | no |
| <a name="input_ldap_password"></a> [ldap\_password](#input\_ldap\_password) | LDAP bind password | `string` | `"readonly"` | no |
| <a name="input_ldap_reader_dn"></a> [ldap\_reader\_dn](#input\_ldap\_reader\_dn) | LDAP Reader DN | `string` | `"cn=readonly-account,dc=example,dc=com"` | no |
| <a name="input_ldap_start_tls"></a> [ldap\_start\_tls](#input\_ldap\_start\_tls) | Enable StartTLS for LDAP | `bool` | `true` | no |
| <a name="input_ldap_url"></a> [ldap\_url](#input\_ldap\_url) | LDAP server URL | `string` | `"ldap.example.com:389"` | no |
| <a name="input_logo_url"></a> [logo\_url](#input\_logo\_url) | Custom logo URL | `string` | `"https://www.portainer.io/hubfs/portainer-logo-black.svg"` | no |
| <a name="input_portainer_api_key"></a> [portainer\_api\_key](#input\_portainer\_api\_key) | Default Portainer Admin API Key | `string` | `"ptr_xrP7XWqfZEOoaCJRu5c8qKaWuDtVc2Zb07Q5g22YpS8="` | no |
| <a name="input_portainer_url"></a> [portainer\_url](#input\_portainer\_url) | Default Portainer URL | `string` | `"http://localhost:9000"` | no |
| <a name="input_required_password_length"></a> [required\_password\_length](#input\_required\_password\_length) | Minimum password length for internal auth | `number` | `18` | no |
| <a name="input_snapshot_interval"></a> [snapshot\_interval](#input\_snapshot\_interval) | Interval for snapshots (e.g., 15m) | `string` | `"15m"` | no |
| <a name="input_user_session_timeout"></a> [user\_session\_timeout](#input\_user\_session\_timeout) | Session timeout duration (e.g., 8h) | `string` | `"8h"` | no |
<!-- END_TF_DOCS -->