<!-- BEGIN_TF_DOCS -->


## Providers

| Name | Version |
|------|---------|
| <a name="provider_local"></a> [local](#provider\_local) | 2.5.3 |
| <a name="provider_portainer"></a> [portainer](#provider\_portainer) | 1.13.2 |

## Resources

| Name | Type |
|------|------|
| [local_file.k8s_manifests](https://registry.terraform.io/providers/hashicorp/local/latest/docs/resources/file) | resource |
| [portainer_compose_convert.docker-compose-yml](https://registry.terraform.io/providers/portainer/portainer/latest/docs/resources/compose_convert) | resource |

## Inputs

| Name | Description | Type | Default | Required |
|------|-------------|------|---------|:--------:|
| <a name="input_portainer_api_key"></a> [portainer\_api\_key](#input\_portainer\_api\_key) | Default Portainer Admin API Key | `string` | `"ptr_xrP7XWqfZEOoaCJRu5c8qKaWuDtVc2Zb07Q5g22YpS8="` | no |
| <a name="input_portainer_skip_ssl_verify"></a> [portainer\_skip\_ssl\_verify](#input\_portainer\_skip\_ssl\_verify) | Set to true to skip TLS certificate verification (useful for self-signed certs) | `bool` | `true` | no |
| <a name="input_portainer_url"></a> [portainer\_url](#input\_portainer\_url) | Default Portainer URL | `string` | `"https://localhost:9443"` | no |
<!-- END_TF_DOCS -->