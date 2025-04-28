<!-- BEGIN_TF_DOCS -->


## Providers

| Name | Version |
|------|---------|
| <a name="provider_portainer"></a> [portainer](#provider\_portainer) | 0.1.0 |

## Resources

| Name | Type |
|------|------|
| [portainer_stack_webhook.trigger_my_stack](https://registry.terraform.io/providers/portainer/portainer/latest/docs/resources/stack_webhook) | resource |

## Inputs

| Name | Description | Type | Default | Required |
|------|-------------|------|---------|:--------:|
| <a name="input_portainer_api_key"></a> [portainer\_api\_key](#input\_portainer\_api\_key) | Default Portainer Admin API Key | `string` | n/a | yes |
| <a name="input_portainer_url"></a> [portainer\_url](#input\_portainer\_url) | Default Portainer URL | `string` | `"http://localhost:9000"` | no |
| <a name="input_webhook_id"></a> [webhook\_id](#input\_webhook\_id) | Webhook token used to trigger the stack update. | `string` | `"65001023-9dd7-415f-9cff-358ba0a78463"` | no |
<!-- END_TF_DOCS -->