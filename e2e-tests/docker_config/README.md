<!-- BEGIN_TF_DOCS -->


## Providers

| Name | Version |
|------|---------|
| <a name="provider_portainer"></a> [portainer](#provider\_portainer) | n/a |

## Resources

| Name | Type |
|------|------|
| [portainer_docker_config.example_config](https://registry.terraform.io/providers/portainer/portainer/latest/docs/resources/docker_config) | resource |

## Inputs

| Name | Description | Type | Default | Required |
|------|-------------|------|---------|:--------:|
| <a name="input_config_data"></a> [config\_data](#input\_config\_data) | Base64-encoded data for Docker config | `string` | `"THIS IS NOT A REAL CERTIFICATE\n"` | no |
| <a name="input_config_labels"></a> [config\_labels](#input\_config\_labels) | Map Docker config labels | `map(string)` | <pre>{<br/>  "foo": "bar",<br/>  "property1": "string",<br/>  "property2": "string"<br/>}</pre> | no |
| <a name="input_config_name"></a> [config\_name](#input\_config\_name) | Name Docker config | `string` | `"server.conf"` | no |
| <a name="input_config_templating"></a> [config\_templating](#input\_config\_templating) | Templating configuration | `map(string)` | <pre>{<br/>  "OptionA": "value for driver-specific option A",<br/>  "OptionB": "value for driver-specific option B",<br/>  "name": "some-driver"<br/>}</pre> | no |
| <a name="input_endpoint_id"></a> [endpoint\_id](#input\_endpoint\_id) | ID of the Portainer endpointr | `number` | `3` | no |
| <a name="input_portainer_api_key"></a> [portainer\_api\_key](#input\_portainer\_api\_key) | Default Portainer Admin API Key | `string` | `"ptr_xrP7XWqfZEOoaCJRu5c8qKaWuDtVc2Zb07Q5g22YpS8="` | no |
| <a name="input_portainer_url"></a> [portainer\_url](#input\_portainer\_url) | Default Portainer URL | `string` | `"http://localhost:9000"` | no |
<!-- END_TF_DOCS -->