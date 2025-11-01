<!-- BEGIN_TF_DOCS -->


## Providers

| Name | Version |
|------|---------|
| <a name="provider_portainer"></a> [portainer](#provider\_portainer) | 1.13.2 |

## Resources

| Name | Type |
|------|------|
| [portainer_resource_control.example](https://registry.terraform.io/providers/portainer/portainer/latest/docs/resources/resource_control) | resource |

## Inputs

| Name | Description | Type | Default | Required |
|------|-------------|------|---------|:--------:|
| <a name="input_administrators_only"></a> [administrators\_only](#input\_administrators\_only) | Restrict access to administrators only | `bool` | `false` | no |
| <a name="input_portainer_api_key"></a> [portainer\_api\_key](#input\_portainer\_api\_key) | Default Portainer Admin API Key | `string` | n/a | yes |
| <a name="input_portainer_url"></a> [portainer\_url](#input\_portainer\_url) | Default Portainer URL | `string` | n/a | yes |
| <a name="input_public"></a> [public](#input\_public) | Whether the resource should be public | `bool` | `false` | no |
| <a name="input_resource_id"></a> [resource\_id](#input\_resource\_id) | ID of the Docker/Kubernetes resource to control | `string` | n/a | yes |
| <a name="input_resource_type"></a> [resource\_type](#input\_resource\_type) | Type of the resource (e.g., 1 = container, 2 = volume, etc.) | `number` | n/a | yes |
| <a name="input_teams"></a> [teams](#input\_teams) | List of team IDs allowed to access the resource | `list(number)` | `[]` | no |
| <a name="input_users"></a> [users](#input\_users) | List of user IDs allowed to access the resource | `list(number)` | `[]` | no |
<!-- END_TF_DOCS -->