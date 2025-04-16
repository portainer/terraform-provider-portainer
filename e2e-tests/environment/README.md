<!-- BEGIN_TF_DOCS -->


## Providers

| Name | Version |
|------|---------|
| <a name="provider_portainer"></a> [portainer](#provider\_portainer) | n/a |

## Resources

| Name | Type |
|------|------|
| [portainer_environment.your-host](https://registry.terraform.io/providers/portainer/portainer/latest/docs/resources/environment) | resource |

## Inputs

| Name | Description | Type | Default | Required |
|------|-------------|------|---------|:--------:|
| <a name="input_portainer_api_key"></a> [portainer\_api\_key](#input\_portainer\_api\_key) | Default Portainer Admin API Key | `string` | `"ptr_xrP7XWqfZEOoaCJRu5c8qKaWuDtVc2Zb07Q5g22YpS8="` | no |
| <a name="input_portainer_environment_address"></a> [portainer\_environment\_address](#input\_portainer\_environment\_address) | Portainer environment address | `string` | `"tcp://host:9001"` | no |
| <a name="input_portainer_environment_name"></a> [portainer\_environment\_name](#input\_portainer\_environment\_name) | Portainer environment name | `string` | `"Your test environment name"` | no |
| <a name="input_portainer_environment_type"></a> [portainer\_environment\_type](#input\_portainer\_environment\_type) | Portainer environment type | `number` | `2` | no |
| <a name="input_portainer_url"></a> [portainer\_url](#input\_portainer\_url) | Default Portainer URL | `string` | `"http://localhost:9000"` | no |
| <a name="input_tls_enabled"></a> [tls\_enabled](#input\_tls\_enabled) | Enable TLS for the agent connection | `bool` | `false` | no |
| <a name="input_tls_skip_client_verify"></a> [tls\_skip\_client\_verify](#input\_tls\_skip\_client\_verify) | Skip client certificate verification (insecure) | `bool` | `false` | no |
| <a name="input_tls_skip_verify"></a> [tls\_skip\_verify](#input\_tls\_skip\_verify) | Skip server certificate verification (insecure) | `bool` | `false` | no |
<!-- END_TF_DOCS -->