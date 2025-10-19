<!-- BEGIN_TF_DOCS -->


## Providers

| Name | Version |
|------|---------|
| <a name="provider_portainer"></a> [portainer](#provider\_portainer) | 1.13.2 |

## Resources

| Name | Type |
|------|------|
| [portainer_webhook_execute.test_token](https://registry.terraform.io/providers/portainer/portainer/latest/docs/resources/webhook_execute) | resource |

## Inputs

| Name | Description | Type | Default | Required |
|------|-------------|------|---------|:--------:|
| <a name="input_portainer_api_key"></a> [portainer\_api\_key](#input\_portainer\_api\_key) | Default Portainer Admin API Key | `string` | n/a | yes |
| <a name="input_portainer_url"></a> [portainer\_url](#input\_portainer\_url) | Default Portainer URL | `string` | n/a | yes |
| <a name="input_stack_id"></a> [stack\_id](#input\_stack\_id) | Stack ID to trigger git update | `string` | n/a | yes |
| <a name="input_webhook_token"></a> [webhook\_token](#input\_webhook\_token) | Webhook token to trigger service restart | `string` | n/a | yes |
<!-- END_TF_DOCS -->