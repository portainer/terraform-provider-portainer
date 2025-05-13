<!-- BEGIN_TF_DOCS -->


## Providers

| Name | Version |
|------|---------|
| <a name="provider_portainer"></a> [portainer](#provider\_portainer) | n/a |

## Resources

| Name | Type |
|------|------|
| [portainer_edge_group.example_static](https://registry.terraform.io/providers/portainer/portainer/latest/docs/resources/edge_group) | resource |
| [portainer_edge_stack.string_example](https://registry.terraform.io/providers/portainer/portainer/latest/docs/resources/edge_stack) | resource |

## Inputs

| Name | Description | Type | Default | Required |
|------|-------------|------|---------|:--------:|
| <a name="input_edge_group_dynamic"></a> [edge\_group\_dynamic](#input\_edge\_group\_dynamic) | Whether the edge group is dynamic | `bool` | `false` | no |
| <a name="input_edge_group_name"></a> [edge\_group\_name](#input\_edge\_group\_name) | Name of the edge group | `string` | `"static-group"` | no |
| <a name="input_edge_group_partial_match"></a> [edge\_group\_partial\_match](#input\_edge\_group\_partial\_match) | Whether to use partial match when dynamic = true | `bool` | `false` | no |
| <a name="input_edge_group_tag_ids"></a> [edge\_group\_tag\_ids](#input\_edge\_group\_tag\_ids) | List of tag IDs used for dynamic matching | `list(number)` | `[]` | no |
| <a name="input_edge_stack_deployment_type"></a> [edge\_stack\_deployment\_type](#input\_edge\_stack\_deployment\_type) | Deployment type (0 = Compose, 1 = Kubernetes) | `number` | `0` | no |
| <a name="input_edge_stack_file_path_in_repository"></a> [edge\_stack\_file\_path\_in\_repository](#input\_edge\_stack\_file\_path\_in\_repository) | Inline file content for the template (YAML/Compose) | `string` | `"docker-compose.agent.yml"` | no |
| <a name="input_edge_stack_name"></a> [edge\_stack\_name](#input\_edge\_stack\_name) | Name of the Portainer Edge Stack | `string` | `"example-edge-stack"` | no |
| <a name="input_edge_stack_registries"></a> [edge\_stack\_registries](#input\_edge\_stack\_registries) | List of registry IDs | `list(number)` | `[]` | no |
| <a name="input_edge_stack_repository_url"></a> [edge\_stack\_repository\_url](#input\_edge\_stack\_repository\_url) | Inline stack file content for the Edge Stack | `string` | `"https://github.com/portainer/terraform-provider-portainer"` | no |
| <a name="input_portainer_api_key"></a> [portainer\_api\_key](#input\_portainer\_api\_key) | Default Portainer Admin API Key | `string` | `"ptr_xrP7XWqfZEOoaCJRu5c8qKaWuDtVc2Zb07Q5g22YpS8="` | no |
| <a name="input_portainer_skip_ssl_verify"></a> [portainer\_skip\_ssl\_verify](#input\_portainer\_skip\_ssl\_verify) | Set to true to skip TLS certificate verification (useful for self-signed certs) | `bool` | `true` | no |
| <a name="input_portainer_url"></a> [portainer\_url](#input\_portainer\_url) | Default Portainer URL | `string` | `"https://localhost:9443"` | no |
<!-- END_TF_DOCS -->