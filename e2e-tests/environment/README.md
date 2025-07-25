<!-- BEGIN_TF_DOCS -->


## Providers

| Name | Version |
|------|---------|
| <a name="provider_portainer"></a> [portainer](#provider\_portainer) | 0.1.0 |

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
| <a name="input_portainer_skip_ssl_verify"></a> [portainer\_skip\_ssl\_verify](#input\_portainer\_skip\_ssl\_verify) | Set to true to skip TLS certificate verification (useful for self-signed certs) | `bool` | `true` | no |
| <a name="input_portainer_url"></a> [portainer\_url](#input\_portainer\_url) | Default Portainer URL | `string` | `"https://localhost:9443"` | no |

## Outputs

| Name | Description |
|------|-------------|
| <a name="output_edge_id"></a> [edge\_id](#output\_edge\_id) | n/a |
| <a name="output_edge_key"></a> [edge\_key](#output\_edge\_key) | n/a |
<!-- END_TF_DOCS -->