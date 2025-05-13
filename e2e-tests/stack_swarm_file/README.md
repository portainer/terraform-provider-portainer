<!-- BEGIN_TF_DOCS -->


## Providers

| Name | Version |
|------|---------|
| <a name="provider_portainer"></a> [portainer](#provider\_portainer) | n/a |

## Resources

| Name | Type |
|------|------|
| [portainer_stack.swarm_file](https://registry.terraform.io/providers/portainer/portainer/latest/docs/resources/stack) | resource |

## Inputs

| Name | Description | Type | Default | Required |
|------|-------------|------|---------|:--------:|
| <a name="input_portainer_api_key"></a> [portainer\_api\_key](#input\_portainer\_api\_key) | Default Portainer Admin API Key | `string` | `"ptr_xrP7XWqfZEOoaCJRu5c8qKaWuDtVc2Zb07Q5g22YpS8="` | no |
| <a name="input_portainer_skip_ssl_verify"></a> [portainer\_skip\_ssl\_verify](#input\_portainer\_skip\_ssl\_verify) | Set to true to skip TLS certificate verification (useful for self-signed certs) | `bool` | `true` | no |
| <a name="input_portainer_url"></a> [portainer\_url](#input\_portainer\_url) | Default Portainer URL | `string` | `"https://localhost:9443"` | no |
| <a name="input_stack_deployment_type"></a> [stack\_deployment\_type](#input\_stack\_deployment\_type) | Deployment type: standalone, swarm, or kubernetes | `string` | `"swarm"` | no |
| <a name="input_stack_endpoint_id"></a> [stack\_endpoint\_id](#input\_stack\_endpoint\_id) | Portainer environment/endpoint ID | `number` | `3` | no |
| <a name="input_stack_env_name"></a> [stack\_env\_name](#input\_stack\_env\_name) | Environment variable name | `string` | `"MY_VAR"` | no |
| <a name="input_stack_env_value"></a> [stack\_env\_value](#input\_stack\_env\_value) | Environment variable value | `string` | `"value"` | no |
| <a name="input_stack_file_path"></a> [stack\_file\_path](#input\_stack\_file\_path) | Inline Docker Compose file content | `string` | `"nginx.yml"` | no |
| <a name="input_stack_method"></a> [stack\_method](#input\_stack\_method) | Creation method: string, file, repository, or url | `string` | `"string"` | no |
| <a name="input_stack_name"></a> [stack\_name](#input\_stack\_name) | Name of the stack | `string` | `"nginx-swarm-file"` | no |
<!-- END_TF_DOCS -->