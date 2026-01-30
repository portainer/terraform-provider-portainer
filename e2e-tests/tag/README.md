<!-- BEGIN_TF_DOCS -->


## Providers

| Name | Version |
|------|---------|
| <a name="provider_portainer"></a> [portainer](#provider\_portainer) | 1.13.2 |

## Resources

| Name | Type |
|------|------|
| [portainer_tag.your-tag](https://registry.terraform.io/providers/portainer/portainer/latest/docs/resources/tag) | resource |
| [portainer_tag.test_lookup](https://registry.terraform.io/providers/portainer/portainer/latest/docs/data-sources/tag) | data source |

## Inputs

| Name | Description | Type | Default | Required |
|------|-------------|------|---------|:--------:|
| <a name="input_portainer_api_key"></a> [portainer\_api\_key](#input\_portainer\_api\_key) | Default Portainer Admin API Key | `string` | `"ptr_xrP7XWqfZEOoaCJRu5c8qKaWuDtVc2Zb07Q5g22YpS8="` | no |
| <a name="input_portainer_skip_ssl_verify"></a> [portainer\_skip\_ssl\_verify](#input\_portainer\_skip\_ssl\_verify) | Set to true to skip TLS certificate verification (useful for self-signed certs) | `bool` | `true` | no |
| <a name="input_portainer_tag_name"></a> [portainer\_tag\_name](#input\_portainer\_tag\_name) | Portainer tag name | `string` | `"your-tag"` | no |
| <a name="input_portainer_url"></a> [portainer\_url](#input\_portainer\_url) | Default Portainer URL | `string` | `"https://localhost:9443"` | no |

## Outputs

| Name | Description |
|------|-------------|
| <a name="output_found_tag_id"></a> [found\_tag\_id](#output\_found\_tag\_id) | n/a |
<!-- END_TF_DOCS -->