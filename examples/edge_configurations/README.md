<!-- BEGIN_TF_DOCS -->


## Providers

| Name | Version |
|------|---------|
| <a name="provider_portainer"></a> [portainer](#provider\_portainer) | 0.1.0 |

## Resources

| Name | Type |
|------|------|
| [portainer_edge_configurations.example](https://registry.terraform.io/providers/portainer/portainer/latest/docs/resources/edge_configurations) | resource |

## Inputs

| Name | Description | Type | Default | Required |
|------|-------------|------|---------|:--------:|
| <a name="input_edge_config_base_dir"></a> [edge\_config\_base\_dir](#input\_edge\_config\_base\_dir) | n/a | `string` | `"/etc/some/path/of/edge/config"` | no |
| <a name="input_edge_config_category"></a> [edge\_config\_category](#input\_edge\_config\_category) | n/a | `string` | `"configuration"` | no |
| <a name="input_edge_config_file_path"></a> [edge\_config\_file\_path](#input\_edge\_config\_file\_path) | n/a | `string` | `"config.zip"` | no |
| <a name="input_edge_config_name"></a> [edge\_config\_name](#input\_edge\_config\_name) | Name of the Edge configuration | `string` | `"Test Edge Config"` | no |
| <a name="input_edge_config_type"></a> [edge\_config\_type](#input\_edge\_config\_type) | n/a | `string` | `"general"` | no |
| <a name="input_edge_group_ids"></a> [edge\_group\_ids](#input\_edge\_group\_ids) | n/a | `list(number)` | <pre>[<br/>  1<br/>]</pre> | no |
| <a name="input_portainer_api_key"></a> [portainer\_api\_key](#input\_portainer\_api\_key) | Default Portainer Admin API Key | `string` | n/a | yes |
| <a name="input_portainer_url"></a> [portainer\_url](#input\_portainer\_url) | Default Portainer URL | `string` | n/a | yes |
<!-- END_TF_DOCS -->