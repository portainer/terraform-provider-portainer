<!-- BEGIN_TF_DOCS -->


## Providers

| Name | Version |
|------|---------|
| <a name="provider_portainer"></a> [portainer](#provider\_portainer) | n/a |

## Resources

| Name | Type |
|------|------|
| [portainer_stack.swarm_string](https://registry.terraform.io/providers/portainer/portainer/latest/docs/resources/stack) | resource |

## Inputs

| Name | Description | Type | Default | Required |
|------|-------------|------|---------|:--------:|
| <a name="input_portainer_api_key"></a> [portainer\_api\_key](#input\_portainer\_api\_key) | Default Portainer Admin API Key | `string` | `"ptr_xrP7XWqfZEOoaCJRu5c8qKaWuDtVc2Zb07Q5g22YpS8="` | no |
| <a name="input_portainer_url"></a> [portainer\_url](#input\_portainer\_url) | Default Portainer URL | `string` | `"http://localhost:9000"` | no |
| <a name="input_stack_deployment_type"></a> [stack\_deployment\_type](#input\_stack\_deployment\_type) | Deployment type of the stack (e.g., 'swarm') | `string` | `"swarm"` | no |
| <a name="input_stack_endpoint_id"></a> [stack\_endpoint\_id](#input\_stack\_endpoint\_id) | ID of the Portainer endpoint | `number` | `3` | no |
| <a name="input_stack_env_name"></a> [stack\_env\_name](#input\_stack\_env\_name) | Name of the environment variable | `string` | `"MY_VAR"` | no |
| <a name="input_stack_env_value"></a> [stack\_env\_value](#input\_stack\_env\_value) | Value of the environment variable | `string` | `"value"` | no |
| <a name="input_stack_file_content"></a> [stack\_file\_content](#input\_stack\_file\_content) | The content of the docker-compose file | `string` | `"version: \"3\"\nservices:\n  web:\n    image: nginx\n"` | no |
| <a name="input_stack_method"></a> [stack\_method](#input\_stack\_method) | Method used to deploy the stack (e.g., 'string', 'repository') | `string` | `"string"` | no |
| <a name="input_stack_name"></a> [stack\_name](#input\_stack\_name) | Name of the Portainer stack | `string` | `"your-swarm-string"` | no |
<!-- END_TF_DOCS -->