<!-- BEGIN_TF_DOCS -->


## Providers

| Name | Version |
|------|---------|
| <a name="provider_portainer"></a> [portainer](#provider\_portainer) | n/a |

## Resources

| Name | Type |
|------|------|
| [portainer_edge_stack.string_example](https://registry.terraform.io/providers/portainer/portainer/latest/docs/resources/edge_stack) | resource |

## Inputs

| Name | Description | Type | Default | Required |
|------|-------------|------|---------|:--------:|
| <a name="input_edge_stack_deployment_type"></a> [edge\_stack\_deployment\_type](#input\_edge\_stack\_deployment\_type) | Deployment type (0 = Compose, 1 = Kubernetes) | `number` | `0` | no |
| <a name="input_edge_stack_edge_groups"></a> [edge\_stack\_edge\_groups](#input\_edge\_stack\_edge\_groups) | List of Edge Group IDs | `list(number)` | `[]` | no |
| <a name="input_edge_stack_file_content"></a> [edge\_stack\_file\_content](#input\_edge\_stack\_file\_content) | Inline stack file content for the Edge Stack | `string` | `"version: '3'\nservices:\n  hello-world:\n    image: hello-world\n"` | no |
| <a name="input_edge_stack_name"></a> [edge\_stack\_name](#input\_edge\_stack\_name) | Name of the Portainer Edge Stack | `string` | `"example-edge-stack"` | no |
| <a name="input_edge_stack_registries"></a> [edge\_stack\_registries](#input\_edge\_stack\_registries) | List of registry IDs | `list(number)` | `[]` | no |
| <a name="input_edge_stack_use_manifest_namespaces"></a> [edge\_stack\_use\_manifest\_namespaces](#input\_edge\_stack\_use\_manifest\_namespaces) | Whether to use manifest namespaces | `bool` | `false` | no |
| <a name="input_portainer_api_key"></a> [portainer\_api\_key](#input\_portainer\_api\_key) | Default Portainer Admin API Key | `string` | `"ptr_xrP7XWqfZEOoaCJRu5c8qKaWuDtVc2Zb07Q5g22YpS8="` | no |
| <a name="input_portainer_url"></a> [portainer\_url](#input\_portainer\_url) | Default Portainer URL | `string` | `"http://localhost:9000"` | no |
<!-- END_TF_DOCS -->