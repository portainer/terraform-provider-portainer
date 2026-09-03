# Data Source Documentation: `portainer_settings_public`

# portainer_settings_public
The `portainer_settings_public` data source reads the settings Portainer exposes without authentication — how the instance expects users to log in, whether Edge Compute is on, and whether it has been set up at all.

## Example Usage

```hcl
data "portainer_settings_public" "this" {}

# Edge environments and Edge stacks need the feature switched on.
resource "portainer_environment" "edge" {
  count = data.portainer_settings_public.this.enable_edge_compute_features ? 1 : 0

  name                = "edge-device"
  environment_address = "https://portainer.example.com"
  type                = 4
}
```

## Arguments Reference

This data source takes no arguments.

## Attributes Reference

| Name                           | Type   | Description                                                                       |
|--------------------------------|--------|-----------------------------------------------------------------------------------|
| `authentication_method`        | number | 1 = internal, 2 = LDAP, 3 = OAuth.                                                 |
| `enable_edge_compute_features` | bool   | Whether Edge Compute features are enabled.                                          |
| `required_password_length`     | number | Minimum password length enforced for internal accounts.                             |
| `requires_setup_token`         | bool   | Whether the instance still demands a setup token, meaning it is uninitialised.       |
| `kubeconfig_expiry`            | string | Lifetime of generated kubeconfig files, `0` meaning they never expire.               |
| `logo_url`                     | string | Custom logo URL, empty when Portainer's own logo is used.                            |
| `oauth_login_uri`              | string | URI users are sent to for OAuth login.                                               |
| `oauth_logout_uri`             | string | URI users are sent to on OAuth logout.                                               |
| `team_sync`                    | bool   | Whether team membership is synchronised from the external provider.                  |
| `is_docker_desktop_extension`  | bool   | Whether Portainer runs as the Docker Desktop extension.                              |
