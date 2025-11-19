<!-- BEGIN_TF_DOCS -->


## Providers

| Name | Version |
|------|---------|
| <a name="provider_portainer"></a> [portainer](#provider\_portainer) | n/a |

## Resources

| Name | Type |
|------|------|
| [portainer_docker_secret.example_secret](https://registry.terraform.io/providers/portainer/portainer/latest/docs/resources/docker_secret) | resource |
| [portainer_resource_control.secret_access](https://registry.terraform.io/providers/portainer/portainer/latest/docs/resources/resource_control) | resource |
| [portainer_team.your_example_team](https://registry.terraform.io/providers/portainer/portainer/latest/docs/resources/team) | resource |

## Inputs

| Name | Description | Type | Default | Required |
|------|-------------|------|---------|:--------:|
| <a name="input_endpoint_id"></a> [endpoint\_id](#input\_endpoint\_id) | ID of the Portainer endpoint | `number` | `3` | no |
| <a name="input_portainer_api_key"></a> [portainer\_api\_key](#input\_portainer\_api\_key) | Default Portainer Admin API Key | `string` | `"ptr_xrP7XWqfZEOoaCJRu5c8qKaWuDtVc2Zb07Q5g22YpS8="` | no |
| <a name="input_portainer_skip_ssl_verify"></a> [portainer\_skip\_ssl\_verify](#input\_portainer\_skip\_ssl\_verify) | Set to true to skip TLS certificate verification (useful for self-signed certs) | `bool` | `true` | no |
| <a name="input_portainer_team_name"></a> [portainer\_team\_name](#input\_portainer\_team\_name) | Portainer Team Name | `string` | `"your-team-test"` | no |
| <a name="input_portainer_url"></a> [portainer\_url](#input\_portainer\_url) | Default Portainer URL | `string` | `"https://localhost:9443"` | no |
| <a name="input_resource_control_administrators_only"></a> [resource\_control\_administrators\_only](#input\_resource\_control\_administrators\_only) | Only administrators can access the resource | `bool` | `false` | no |
| <a name="input_resource_control_public"></a> [resource\_control\_public](#input\_resource\_control\_public) | Whether the resource is public | `bool` | `false` | no |
| <a name="input_resource_control_type"></a> [resource\_control\_type](#input\_resource\_control\_type) | Portainer ResourceControl type | `number` | `5` | no |
| <a name="input_secret_data"></a> [secret\_data](#input\_secret\_data) | Base64ncoded data for secret | `string` | `"THIS IS NOT A REAL CERTIFICATE\n"` | no |
| <a name="input_secret_labels"></a> [secret\_labels](#input\_secret\_labels) | Map Docker secret labels | `map(string)` | <pre>{<br/>  "com.example.some-label": "some-value"<br/>}</pre> | no |
| <a name="input_secret_name"></a> [secret\_name](#input\_secret\_name) | Name of Docker secret | `string` | `"app-key.crt"` | no |
| <a name="input_secret_templating"></a> [secret\_templating](#input\_secret\_templating) | Template configuration | `map(string)` | <pre>{<br/>  "OptionA": "value for driver-specific option A",<br/>  "name": "some-driver"<br/>}</pre> | no |
<!-- END_TF_DOCS -->