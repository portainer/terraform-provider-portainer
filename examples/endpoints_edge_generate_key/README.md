<!-- BEGIN_TF_DOCS -->


## Providers

| Name | Version |
|------|---------|
| <a name="provider_portainer"></a> [portainer](#provider\_portainer) | 1.13.2 |

## Resources

| Name | Type |
|------|------|
| [portainer_endpoints_edge_generate_key.generated](https://registry.terraform.io/providers/portainer/portainer/latest/docs/resources/endpoints_edge_generate_key) | resource |

## Inputs

| Name | Description | Type | Default | Required |
|------|-------------|------|---------|:--------:|
| <a name="input_portainer_api_key"></a> [portainer\_api\_key](#input\_portainer\_api\_key) | Default Portainer Admin API Key | `string` | n/a | yes |
| <a name="input_portainer_url"></a> [portainer\_url](#input\_portainer\_url) | Default Portainer URL | `string` | n/a | yes |

## Outputs

| Name | Description |
|------|-------------|
| <a name="output_edge_key"></a> [edge\_key](#output\_edge\_key) | n/a |
<!-- END_TF_DOCS -->