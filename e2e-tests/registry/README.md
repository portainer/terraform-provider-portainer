<!-- BEGIN_TF_DOCS -->


## Providers

| Name | Version |
|------|---------|
| <a name="provider_portainer"></a> [portainer](#provider\_portainer) | n/a |

## Resources

| Name | Type |
|------|------|
| [portainer_registry.registry](https://registry.terraform.io/providers/portainer/portainer/latest/docs/resources/registry) | resource |

## Inputs

| Name | Description | Type | Default | Required |
|------|-------------|------|---------|:--------:|
| <a name="input_portainer_api_key"></a> [portainer\_api\_key](#input\_portainer\_api\_key) | Default Portainer Admin API Key | `string` | `"ptr_xrP7XWqfZEOoaCJRu5c8qKaWuDtVc2Zb07Q5g22YpS8="` | no |
| <a name="input_portainer_registry_authentication"></a> [portainer\_registry\_authentication](#input\_portainer\_registry\_authentication) | Required use authentication | `bool` | `false` | no |
| <a name="input_portainer_registry_name"></a> [portainer\_registry\_name](#input\_portainer\_registry\_name) | Custom Registry | `string` | `"DockerHub"` | no |
| <a name="input_portainer_registry_type"></a> [portainer\_registry\_type](#input\_portainer\_registry\_type) | Type registry | `number` | `3` | no |
| <a name="input_portainer_registry_url"></a> [portainer\_registry\_url](#input\_portainer\_registry\_url) | URL adresa registru | `string` | `"test-reegistry-docker.com"` | no |
| <a name="input_portainer_skip_ssl_verify"></a> [portainer\_skip\_ssl\_verify](#input\_portainer\_skip\_ssl\_verify) | Set to true to skip TLS certificate verification (useful for self-signed certs) | `bool` | `true` | no |
| <a name="input_portainer_url"></a> [portainer\_url](#input\_portainer\_url) | Default Portainer URL | `string` | `"https://localhost:9443"` | no |
<!-- END_TF_DOCS -->