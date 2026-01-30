<!-- BEGIN_TF_DOCS -->


## Providers

| Name | Version |
|------|---------|
| <a name="provider_portainer"></a> [portainer](#provider\_portainer) | 0.1.0 |

## Resources

| Name | Type |
|------|------|
| [portainer_endpoint_group.test](https://registry.terraform.io/providers/portainer/portainer/latest/docs/resources/endpoint_group) | resource |
| [portainer_endpoint_group_access.test](https://registry.terraform.io/providers/portainer/portainer/latest/docs/resources/endpoint_group_access) | resource |
| [portainer_team.test](https://registry.terraform.io/providers/portainer/portainer/latest/docs/resources/team) | resource |

## Inputs

| Name | Description | Type | Default | Required |
|------|-------------|------|---------|:--------:|
| <a name="input_endpoint_group_description"></a> [endpoint\_group\_description](#input\_endpoint\_group\_description) | Description of the endpoint group | `string` | `"E2E Test Group for Access Control"` | no |
| <a name="input_endpoint_group_name"></a> [endpoint\_group\_name](#input\_endpoint\_group\_name) | Name of the endpoint group | `string` | `"e2e-access-group"` | no |
| <a name="input_portainer_api_key"></a> [portainer\_api\_key](#input\_portainer\_api\_key) | Default Portainer Admin API Key | `string` | `"ptr_xrP7XWqfZEOoaCJRu5c8qKaWuDtVc2Zb07Q5g22YpS8="` | no |
| <a name="input_portainer_skip_ssl_verify"></a> [portainer\_skip\_ssl\_verify](#input\_portainer\_skip\_ssl\_verify) | Skip SSL verification | `bool` | `true` | no |
| <a name="input_portainer_url"></a> [portainer\_url](#input\_portainer\_url) | Default Portainer URL | `string` | `"https://localhost:9443"` | no |
| <a name="input_team_name"></a> [team\_name](#input\_team\_name) | Name of the team | `string` | `"e2e-access-team"` | no |
<!-- END_TF_DOCS -->