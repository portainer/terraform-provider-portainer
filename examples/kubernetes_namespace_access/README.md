<!-- BEGIN_TF_DOCS -->


## Providers

| Name | Version |
|------|---------|
| <a name="provider_kubernetes"></a> [kubernetes](#provider\_kubernetes) | n/a |

## Resources

| Name | Type |
|------|------|
| [kubernetes_namespace_access.test](https://registry.terraform.io/providers/hashicorp/kubernetes/latest/docs/resources/namespace_access) | resource |

## Inputs

| Name | Description | Type | Default | Required |
|------|-------------|------|---------|:--------:|
| <a name="input_environment_id"></a> [environment\_id](#input\_environment\_id) | ID of the Portainer environment (Kubernetes endpoint). | `number` | n/a | yes |
| <a name="input_namespace_name"></a> [namespace\_name](#input\_namespace\_name) | Name of the Kubernetes namespace to create. | `string` | n/a | yes |
| <a name="input_portainer_api_key"></a> [portainer\_api\_key](#input\_portainer\_api\_key) | Default Portainer Admin API Key | `string` | n/a | yes |
| <a name="input_portainer_url"></a> [portainer\_url](#input\_portainer\_url) | Default Portainer URL | `string` | n/a | yes |
| <a name="input_teams_to_add"></a> [teams\_to\_add](#input\_teams\_to\_add) | List of team IDs to grant access to the namespace | `list(number)` | <pre>[<br/>  7<br/>]</pre> | no |
| <a name="input_teams_to_remove"></a> [teams\_to\_remove](#input\_teams\_to\_remove) | List of team IDs to revoke access from the namespace | `list(number)` | `[]` | no |
| <a name="input_users_to_add"></a> [users\_to\_add](#input\_users\_to\_add) | List of user IDs to grant access to the namespace | `list(number)` | <pre>[<br/>  3,<br/>  5<br/>]</pre> | no |
| <a name="input_users_to_remove"></a> [users\_to\_remove](#input\_users\_to\_remove) | List of user IDs to revoke access from the namespace | `list(number)` | `[]` | no |
<!-- END_TF_DOCS -->