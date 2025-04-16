<!-- BEGIN_TF_DOCS -->


## Providers

| Name | Version |
|------|---------|
| <a name="provider_portainer"></a> [portainer](#provider\_portainer) | n/a |

## Resources

| Name | Type |
|------|------|
| [portainer_endpoint_service_update.force_update](https://registry.terraform.io/providers/portainer/portainer/latest/docs/resources/endpoint_service_update) | resource |

## Inputs

| Name | Description | Type | Default | Required |
|------|-------------|------|---------|:--------:|
| <a name="input_endpoint_id"></a> [endpoint\_id](#input\_endpoint\_id) | ID of the Portainer endpoint | `number` | `3` | no |
| <a name="input_portainer_api_key"></a> [portainer\_api\_key](#input\_portainer\_api\_key) | Default Portainer Admin API Key | `string` | n/a | yes |
| <a name="input_portainer_url"></a> [portainer\_url](#input\_portainer\_url) | Default Portainer URL | `string` | n/a | yes |
| <a name="input_pull_image"></a> [pull\_image](#input\_pull\_image) | Whether to pull the latest image before updating the service | `bool` | `true` | no |
| <a name="input_service_name"></a> [service\_name](#input\_service\_name) | Name of the Docker service to force update | `string` | `"my_service"` | no |
<!-- END_TF_DOCS -->