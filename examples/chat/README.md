<!-- BEGIN_TF_DOCS -->


## Providers

| Name | Version |
|------|---------|
| <a name="provider_portainer"></a> [portainer](#provider\_portainer) | 1.13.2 |

## Resources

| Name | Type |
|------|------|
| [portainer_chat.test](https://registry.terraform.io/providers/portainer/portainer/latest/docs/resources/chat) | resource |

## Inputs

| Name | Description | Type | Default | Required |
|------|-------------|------|---------|:--------:|
| <a name="input_chat_context"></a> [chat\_context](#input\_chat\_context) | The context for the chat query (e.g., 'environment\_aware') | `string` | `"environment_aware"` | no |
| <a name="input_chat_environment_id"></a> [chat\_environment\_id](#input\_chat\_environment\_id) | ID of the Portainer environment where the chat applies | `number` | n/a | yes |
| <a name="input_chat_message"></a> [chat\_message](#input\_chat\_message) | The message or query to send to the Portainer chat endpoint | `string` | n/a | yes |
| <a name="input_chat_model"></a> [chat\_model](#input\_chat\_model) | OpenAI model to use (e.g., 'gpt-3.5-turbo', 'gpt-4') | `string` | `"gpt-3.5-turbo"` | no |
| <a name="input_portainer_api_key"></a> [portainer\_api\_key](#input\_portainer\_api\_key) | Default Portainer Admin API Key | `string` | n/a | yes |
| <a name="input_portainer_url"></a> [portainer\_url](#input\_portainer\_url) | Default Portainer URL | `string` | n/a | yes |

## Outputs

| Name | Description |
|------|-------------|
| <a name="output_chat_message"></a> [chat\_message](#output\_chat\_message) | n/a |
| <a name="output_chat_yaml"></a> [chat\_yaml](#output\_chat\_yaml) | n/a |
<!-- END_TF_DOCS -->