<!-- BEGIN_TF_DOCS -->


## Providers

| Name | Version |
|------|---------|
| <a name="provider_portainer"></a> [portainer](#provider\_portainer) | n/a |

## Resources

| Name | Type |
|------|------|
| [portainer_open_amt.enable](https://registry.terraform.io/providers/portainer/portainer/latest/docs/resources/open_amt) | resource |

## Inputs

| Name | Description | Type | Default | Required |
|------|-------------|------|---------|:--------:|
| <a name="input_cert_file_name"></a> [cert\_file\_name](#input\_cert\_file\_name) | Name of the PFX certificate file | `string` | n/a | yes |
| <a name="input_cert_file_password"></a> [cert\_file\_password](#input\_cert\_file\_password) | Password for the PFX certificate | `string` | n/a | yes |
| <a name="input_cert_file_path"></a> [cert\_file\_path](#input\_cert\_file\_path) | Path to the local PFX certificate file (base64 encoded via filebase64) | `string` | n/a | yes |
| <a name="input_domain_name"></a> [domain\_name](#input\_domain\_name) | Domain name for OpenAMT | `string` | n/a | yes |
| <a name="input_enabled"></a> [enabled](#input\_enabled) | Enable or disable OpenAMT | `bool` | `true` | no |
| <a name="input_mpspassword"></a> [mpspassword](#input\_mpspassword) | Password for MPS server | `string` | n/a | yes |
| <a name="input_mpsserver"></a> [mpsserver](#input\_mpsserver) | URL of the MPS (Management Presence Server) | `string` | n/a | yes |
| <a name="input_mpsuser"></a> [mpsuser](#input\_mpsuser) | Username for MPS server | `string` | n/a | yes |
| <a name="input_portainer_api_key"></a> [portainer\_api\_key](#input\_portainer\_api\_key) | Default Portainer Admin API Key | `string` | n/a | yes |
| <a name="input_portainer_url"></a> [portainer\_url](#input\_portainer\_url) | Default Portainer URL | `string` | n/a | yes |
<!-- END_TF_DOCS -->