<!-- BEGIN_TF_DOCS -->


## Providers

| Name | Version |
|------|---------|
| <a name="provider_portainer"></a> [portainer](#provider\_portainer) | 1.13.2 |

## Resources

| Name | Type |
|------|------|
| [portainer_edge_group.example_dynamic](https://registry.terraform.io/providers/portainer/portainer/latest/docs/resources/edge_group) | resource |

## Inputs

| Name | Description | Type | Default | Required |
|------|-------------|------|---------|:--------:|
| <a name="input_edge_group_dynamic"></a> [edge\_group\_dynamic](#input\_edge\_group\_dynamic) | Whether the edge group is dynamic | `bool` | n/a | yes |
| <a name="input_edge_group_name"></a> [edge\_group\_name](#input\_edge\_group\_name) | Name of the edge group | `string` | n/a | yes |
| <a name="input_edge_group_partial_match"></a> [edge\_group\_partial\_match](#input\_edge\_group\_partial\_match) | Whether to use partial match when dynamic = true | `bool` | n/a | yes |
| <a name="input_edge_group_tag_ids"></a> [edge\_group\_tag\_ids](#input\_edge\_group\_tag\_ids) | List of tag IDs used for dynamic matching | `list(number)` | n/a | yes |
| <a name="input_portainer_api_key"></a> [portainer\_api\_key](#input\_portainer\_api\_key) | Default Portainer Admin API Key | `string` | n/a | yes |
| <a name="input_portainer_url"></a> [portainer\_url](#input\_portainer\_url) | Default Portainer URL | `string` | n/a | yes |
<!-- END_TF_DOCS -->