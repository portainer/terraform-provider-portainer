<!-- BEGIN_TF_DOCS -->


## Providers

| Name | Version |
|------|---------|
| <a name="provider_portainer"></a> [portainer](#provider\_portainer) | 1.13.2 |

## Resources

| Name | Type |
|------|------|
| [portainer_auth.login](https://registry.terraform.io/providers/portainer/portainer/latest/docs/resources/auth) | resource |

## Inputs

| Name | Description | Type | Default | Required |
|------|-------------|------|---------|:--------:|
| <a name="input_portainer_api_key"></a> [portainer\_api\_key](#input\_portainer\_api\_key) | Default Portainer Admin API Key | `string` | `"some-fake-api-token"` | no |
| <a name="input_portainer_password"></a> [portainer\_password](#input\_portainer\_password) | Portainer password | `string` | `"password123456789"` | no |
| <a name="input_portainer_skip_ssl_verify"></a> [portainer\_skip\_ssl\_verify](#input\_portainer\_skip\_ssl\_verify) | Set to true to skip TLS certificate verification (useful for self-signed certs) | `bool` | `true` | no |
| <a name="input_portainer_url"></a> [portainer\_url](#input\_portainer\_url) | Default Portainer URL | `string` | `"https://localhost:9443"` | no |
| <a name="input_portainer_username"></a> [portainer\_username](#input\_portainer\_username) | Portainer username | `string` | `"admin"` | no |

## Outputs

| Name | Description |
|------|-------------|
| <a name="output_jwt_token"></a> [jwt\_token](#output\_jwt\_token) | n/a |
<!-- END_TF_DOCS -->