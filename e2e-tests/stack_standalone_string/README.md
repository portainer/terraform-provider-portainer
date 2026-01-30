<!-- BEGIN_TF_DOCS -->


## Providers

| Name | Version |
|------|---------|
| <a name="provider_portainer"></a> [portainer](#provider\_portainer) | 0.1.0 |

## Resources

| Name | Type |
|------|------|
| [portainer_resource_control.standalone_string_rc](https://registry.terraform.io/providers/portainer/portainer/latest/docs/resources/resource_control) | resource |
| [portainer_stack.standalone_string](https://registry.terraform.io/providers/portainer/portainer/latest/docs/resources/stack) | resource |

## Inputs

| Name | Description | Type | Default | Required |
|------|-------------|------|---------|:--------:|
| <a name="input_portainer_api_key"></a> [portainer\_api\_key](#input\_portainer\_api\_key) | Default Portainer Admin API Key | `string` | `"ptr_xrP7XWqfZEOoaCJRu5c8qKaWuDtVc2Zb07Q5g22YpS8="` | no |
| <a name="input_portainer_skip_ssl_verify"></a> [portainer\_skip\_ssl\_verify](#input\_portainer\_skip\_ssl\_verify) | Set to true to skip TLS certificate verification (useful for self-signed certs) | `bool` | `true` | no |
| <a name="input_portainer_url"></a> [portainer\_url](#input\_portainer\_url) | Default Portainer URL | `string` | `"https://localhost:9443"` | no |
| <a name="input_stack_deployment_type"></a> [stack\_deployment\_type](#input\_stack\_deployment\_type) | Deployment type: standalone, swarm, or kubernetes | `string` | `"standalone"` | no |
| <a name="input_stack_endpoint_id"></a> [stack\_endpoint\_id](#input\_stack\_endpoint\_id) | Portainer environment/endpoint ID | `number` | `3` | no |
| <a name="input_stack_env_name"></a> [stack\_env\_name](#input\_stack\_env\_name) | Environment variable name | `string` | `"MY_VAR"` | no |
| <a name="input_stack_env_value"></a> [stack\_env\_value](#input\_stack\_env\_value) | Environment variable value | `string` | `"value"` | no |
| <a name="input_stack_file_content"></a> [stack\_file\_content](#input\_stack\_file\_content) | Inline Docker Compose file content | `string` | `"version: \"3\"\nservices:\n  web:\n    image: nginx\n"` | no |
| <a name="input_stack_method"></a> [stack\_method](#input\_stack\_method) | Creation method: string, file, repository, or url | `string` | `"string"` | no |
| <a name="input_stack_name"></a> [stack\_name](#input\_stack\_name) | Name of the stack | `string` | `"nginx-standalone-string"` | no |
<!-- END_TF_DOCS -->