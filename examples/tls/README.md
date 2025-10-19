<!-- BEGIN_TF_DOCS -->


## Providers

| Name | Version |
|------|---------|
| <a name="provider_portainer"></a> [portainer](#provider\_portainer) | 1.13.2 |

## Resources

| Name | Type |
|------|------|
| [portainer_tls.upload_cert](https://registry.terraform.io/providers/portainer/portainer/latest/docs/resources/tls) | resource |

## Inputs

| Name | Description | Type | Default | Required |
|------|-------------|------|---------|:--------:|
| <a name="input_certificate"></a> [certificate](#input\_certificate) | Type of TLS file to upload: 'cert', 'ca', or 'key' | `string` | `"cert"` | no |
| <a name="input_file_path"></a> [file\_path](#input\_file\_path) | Path to the local TLS file to upload | `string` | `"my-cert.pem"` | no |
| <a name="input_folder"></a> [folder](#input\_folder) | Destination folder in Portainer to store the TLS file | `string` | `"my-endpoint-folder"` | no |
| <a name="input_portainer_api_key"></a> [portainer\_api\_key](#input\_portainer\_api\_key) | Default Portainer Admin API Key | `string` | n/a | yes |
| <a name="input_portainer_url"></a> [portainer\_url](#input\_portainer\_url) | Default Portainer URL | `string` | n/a | yes |
<!-- END_TF_DOCS -->