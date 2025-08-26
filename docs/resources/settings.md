# 🛠 **Resource Documentation: `portainer_settings`**

# portainer_settings
The `portainer_settings` resource allows you to manage global Portainer settings, including authentication, UI branding, telemetry, LDAP, and more.

## Example Usage
```hcl
resource "portainer_settings" "example" {
  authentication_method = 1
  enable_telemetry      = false
  logo_url              = "https://www.portainer.io/hubfs/portainer-logo-black.svg"
  snapshot_interval     = "15m"
  user_session_timeout  = "8h"

  internal_auth_settings {
    required_password_length = 18
  }

  ldap_settings {
    anonymous_mode     = true
    auto_create_users  = true
    password           = "readonly"
    reader_dn          = "cn=readonly-account,dc=example,dc=com"
    start_tls          = true
    url                = "ldap.example.com:389"
  }
}
```

## Lifecycle & Behavior
Settings of Portainer are modify if any of the arguments change by run:
```hcl
terraform apply
```

## Arguments Reference
### Main Attributes
| Name                           | Type     | Required | Description                                                                  |
|--------------------------------|----------|----------|------------------------------------------------------------------------------|
| `authentication_method`        | number   | ✅ yes   | Type of authentication (e.g., `1` = internal, `2` = LDAP, `3` = OAuth)       |
| `enable_telemetry`             | bool     | 🚫 no    | Enable Portainer telemetry                                                   |
| `logo_url`                     | string   | 🚫 no    | URL to custom logo                                                           |
| `snapshot_interval`            | string   | 🚫 no    | How often to run container snapshots (e.g., `"15m"`)                         |
| `templates_url`                | string   | 🚫 no    | URL to the template list JSON                                                |
| `user_session_timeout`         | string   | 🚫 no    | Session expiration time (e.g., `"8h"`)                                       |
| `kubeconfig_expiry`            | string   | 🚫 no    | Expiration time for downloaded Kubeconfigs                                   |
| `kubectl_shell_image`          | string   | 🚫 no    | Image to be used for the kubectl shell UI                                    |
| `helm_repository_url`          | string   | 🚫 no    | Default Helm repository URL                                                  |
| `enable_edge_compute_features` | bool     | 🚫 no    | Enable Edge compute management support                                       |
| `enforce_edge_id`              | bool     | 🚫 no    | Enforce the use of Portainer Edge ID                                         |
| `trust_on_first_connect`       | bool     | 🚫 no    | Automatically trust TLS fingerprint on first connection                      |
| `edge_agent_checkin_interval`  | number   | 🚫 no    | Interval (in seconds) for Edge Agent check-ins                               |
| `disable_kube_roles_sync`      | bool     | ❌ no    | Disable Kubernetes role sync from RBAC                                       |
| `disable_kube_shell`           | bool     | ❌ no    | Disable the Kubectl Shell feature                                            |
| `disable_kubeconfig_download`  | bool     | ❌ no    | Disable downloading of Kubeconfig files                                      |
| `display_donation_header`      | bool     | ❌ no    | Show the donation header in UI                                               |
| `display_external_contributors`| bool     | ❌ no    | Show the list of external contributors in the UI                             |
| `is_docker_desktop_extension`  | bool     | ❌ no    | Whether Portainer is running as Docker Desktop extension                     |

### `global_deployment_options` Block
| Name                        | Type | Required | Description                     |
| --------------------------- | ---- | -------- | ------------------------------- |
| `hide_stacks_functionality` | bool | 🚫 no    | Hide the Stacks UI in Portainer |


### `internal_auth_settings` Block
| Name                      | Type     | Required | Description                              |
|---------------------------|----------|----------|------------------------------------------|
| `required_password_length`| number   | 🚫 no    | Minimum password length for users        |

### `ldap_settings` Block
| Name                    | Type         | Required | Description                                                                               |
| ----------------------- | ------------ | -------- | ------------------------------------------------------------------------------------------|
| `anonymous_mode`        | bool         | 🚫 no    | Use anonymous bind                                                                        |
| `auto_create_users`     | bool         | 🚫 no    | Automatically create users on login                                                       |
| `password`              | string       | 🚫 no    | Password for bind account (use `reader_dn`)                                               |
| `reader_dn`             | string       | 🚫 no    | Reader distinguished name                                                                 |
| `start_tls`             | bool         | 🚫 no    | Enable StartTLS for LDAP connection                                                       |
| `url`                   | string       | ✅ yes    | LDAP server URL, e.g., `ldap.example.com:389`                                            |
| `search_settings`       | list(object) | 🚫 no    | List of user search settings ([nested block](#search_settings-nested-block))              |
| `group_search_settings` | list(object) | 🚫 no    | List of group search configurations ([nested block](#group_search_settings-nested-block)) |
| `tls_config`            | object       | 🚫 no    | TLS configuration for secure LDAP ([nested block](#tls_config-nested-block))              |

#### `search_settings` Nested Block
| Name                  | Type     | Required | Description                            |
|-----------------------|----------|----------|----------------------------------------|
| `base_dn`             | string   | 🚫 no    | Base DN for user search               |
| `filter`              | string   | 🚫 no    | Filter to find user entries           |
| `user_name_attribute` | string   | 🚫 no    | Attribute for usernames (e.g., `uid`) |

#### `group_search_settings` Nested Block
| Name                  | Type     | Required | Description                                  |
|-----------------------|----------|----------|----------------------------------------------|
| `group_attribute`     | string   | 🚫 no    | LDAP attribute representing group membership |
| `group_base_dn`       | string   | 🚫 no    | Base DN for group search                     |
| `group_filter`        | string   | 🚫 no    | Filter to locate groups                      |

#### `tls_config` Nested Block
| Name                  | Type     | Required | Description                   |
|-----------------------|----------|----------|-------------------------------|
| `tls`                 | bool     | 🚫 no    | Enable TLS                    |
| `tls_ca_cert`         | string   | 🚫 no    | Path to CA cert file          |
| `tls_cert`            | string   | 🚫 no    | Path to client cert file      |
| `tls_key`             | string   | 🚫 no    | Path to client key file       |
| `tls_skip_verify`     | bool     | 🚫 no    | Skip certificate verification |

### `oauth_settings` Block
| Name                  | Type           | Required | Description                                                                 |
|-----------------------|----------------|----------|-----------------------------------------------------------------------------|
| `access_token_uri`    | string         | ✅ yes   | OAuth token endpoint                                                        |
| `auth_style`          | number         | 🚫 no    | OAuth auth style (e.g., 0 = auto, 1 = basic, 2 = post)                      |
| `authorization_uri`   | string         | ✅ yes   | OAuth authorization endpoint                                                |
| `client_id`           | string         | ✅ yes   | OAuth client ID                                                             |
| `client_secret`       | string         | ✅ yes   | OAuth client secret                                                         |
| `default_team_id`     | number         | 🚫 no    | ID of default team assigned to new users                                   |
| `kube_secret_key`     | list(number)   | 🚫 no    | List of Kube secret key IDs                                                |
| `logout_uri`          | string         | 🚫 no    | OAuth logout endpoint                                                       |
| `oauth_auto_create_users` | bool      | 🚫 no    | Automatically create users on first login                                  |
| `redirect_uri`        | string         | ✅ yes   | OAuth redirect URI                                                          |
| `resource_uri`        | string         | 🚫 no    | Resource URI for user info                                                 |
| `sso`                 | bool           | 🚫 no    | Enable SSO                                                                  |
| `scopes`              | string         | 🚫 no    | Scopes requested during authentication                                      |
| `user_identifier`     | string         | 🚫 no    | Attribute or claim used to identify users                                  |

### `blacklisted_labels` Block
| Name     | Type   | Required | Description                         |
|----------|--------|----------|-------------------------------------|
| `name`   | string | ✅ yes   | Name of the blacklisted label       |
| `value`  | string | ✅ yes   | Value of the blacklisted label      |

## Attributes Reference

| Name | Description              |
|------|--------------------------|
| `id` | Always `"portainer-settings"` |
