<!-- BEGIN_TF_DOCS -->


## Providers

| Name | Version |
|------|---------|
| <a name="provider_portainer"></a> [portainer](#provider\_portainer) | 1.13.2 |

## Resources

| Name | Type |
|------|------|
| [portainer_ssl.cert_update](https://registry.terraform.io/providers/portainer/portainer/latest/docs/resources/ssl) | resource |

## Inputs

| Name | Description | Type | Default | Required |
|------|-------------|------|---------|:--------:|
| <a name="input_portainer_api_key"></a> [portainer\_api\_key](#input\_portainer\_api\_key) | Default Portainer Admin API Key | `string` | n/a | yes |
| <a name="input_portainer_url"></a> [portainer\_url](#input\_portainer\_url) | Default Portainer URL | `string` | n/a | yes |
| <a name="input_ssl_cert_path"></a> [ssl\_cert\_path](#input\_ssl\_cert\_path) | Path to the SSL certificate file | `string` | `"certs/server.crt"` | no |
| <a name="input_ssl_http_enabled"></a> [ssl\_http\_enabled](#input\_ssl\_http\_enabled) | Whether to enable HTTP access in addition to HTTPS | `bool` | `false` | no |
| <a name="input_ssl_key_path"></a> [ssl\_key\_path](#input\_ssl\_key\_path) | Path to the SSL private key file | `string` | `"certs/server.key"` | no |
<!-- END_TF_DOCS -->