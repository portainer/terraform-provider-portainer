<!-- BEGIN_TF_DOCS -->


## Providers

| Name | Version |
|------|---------|
| <a name="provider_portainer"></a> [portainer](#provider\_portainer) | n/a |

## Resources

| Name | Type |
|------|------|
| [portainer_licenses.example](https://registry.terraform.io/providers/portainer/portainer/latest/docs/resources/licenses) | resource |

## Inputs

| Name | Description | Type | Default | Required |
|------|-------------|------|---------|:--------:|
| <a name="input_license_force"></a> [license\_force](#input\_license\_force) | Whether to force overwrite conflicting licenses | `bool` | `false` | no |
| <a name="input_license_key"></a> [license\_key](#input\_license\_key) | Portainer license key to apply | `string` | n/a | yes |
| <a name="input_portainer_api_key"></a> [portainer\_api\_key](#input\_portainer\_api\_key) | Default Portainer Admin API Key | `string` | n/a | yes |
| <a name="input_portainer_url"></a> [portainer\_url](#input\_portainer\_url) | Default Portainer URL | `string` | n/a | yes |
<!-- END_TF_DOCS -->