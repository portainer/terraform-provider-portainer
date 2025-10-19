<!-- BEGIN_TF_DOCS -->


## Providers

| Name | Version |
|------|---------|
| <a name="provider_portainer"></a> [portainer](#provider\_portainer) | 1.13.2 |

## Resources

| Name | Type |
|------|------|
| [portainer_user.your-user](https://registry.terraform.io/providers/portainer/portainer/latest/docs/resources/user) | resource |

## Inputs

| Name | Description | Type | Default | Required |
|------|-------------|------|---------|:--------:|
| <a name="input_portainer_api_key"></a> [portainer\_api\_key](#input\_portainer\_api\_key) | Default Portainer Admin API Key | `string` | n/a | yes |
| <a name="input_portainer_url"></a> [portainer\_url](#input\_portainer\_url) | Default Portainer URL | `string` | n/a | yes |
| <a name="input_portainer_user_password"></a> [portainer\_user\_password](#input\_portainer\_user\_password) | Portainer password used for resource provisioning | `string` | n/a | yes |
| <a name="input_portainer_user_role"></a> [portainer\_user\_role](#input\_portainer\_user\_role) | Role to assign to the Portainer user | `number` | n/a | yes |
| <a name="input_portainer_user_username"></a> [portainer\_user\_username](#input\_portainer\_user\_username) | Portainer username used for resource provisioning | `string` | n/a | yes |
<!-- END_TF_DOCS -->