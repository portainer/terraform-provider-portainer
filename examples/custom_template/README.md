<!-- BEGIN_TF_DOCS -->


## Providers

| Name | Version |
|------|---------|
| <a name="provider_portainer"></a> [portainer](#provider\_portainer) | n/a |

## Resources

| Name | Type |
|------|------|
| [portainer_custom_template.example_string](https://registry.terraform.io/providers/portainer/portainer/latest/docs/resources/custom_template) | resource |

## Inputs

| Name | Description | Type | Default | Required |
|------|-------------|------|---------|:--------:|
| <a name="input_custom_template_description"></a> [custom\_template\_description](#input\_custom\_template\_description) | Description of the custom template | `string` | n/a | yes |
| <a name="input_custom_template_edge"></a> [custom\_template\_edge](#input\_custom\_template\_edge) | Whether this is an Edge template | `bool` | `false` | no |
| <a name="input_custom_template_file_content"></a> [custom\_template\_file\_content](#input\_custom\_template\_file\_content) | Inline file content for the template (YAML/Compose) | `string` | n/a | yes |
| <a name="input_custom_template_is_compose"></a> [custom\_template\_is\_compose](#input\_custom\_template\_is\_compose) | Is Compose format (true/false) | `bool` | `false` | no |
| <a name="input_custom_template_note"></a> [custom\_template\_note](#input\_custom\_template\_note) | Note that appears in the UI | `string` | n/a | yes |
| <a name="input_custom_template_platform"></a> [custom\_template\_platform](#input\_custom\_template\_platform) | Platform: 1 = linux, 2 = windows | `number` | n/a | yes |
| <a name="input_custom_template_title"></a> [custom\_template\_title](#input\_custom\_template\_title) | Title of the custom template | `string` | n/a | yes |
| <a name="input_custom_template_type"></a> [custom\_template\_type](#input\_custom\_template\_type) | Stack type: 1 = swarm, 2 = compose, 3 = kubernetes | `number` | n/a | yes |
| <a name="input_portainer_api_key"></a> [portainer\_api\_key](#input\_portainer\_api\_key) | Default Portainer Admin API Key | `string` | n/a | yes |
| <a name="input_portainer_url"></a> [portainer\_url](#input\_portainer\_url) | Default Portainer URL | `string` | n/a | yes |
<!-- END_TF_DOCS -->