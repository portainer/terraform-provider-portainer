<!-- BEGIN_TF_DOCS -->


## Providers

| Name | Version |
|------|---------|
| <a name="provider_portainer"></a> [portainer](#provider\_portainer) | n/a |

## Resources

| Name | Type |
|------|------|
| [portainer_kubernetes_delete_object.remove_services](https://registry.terraform.io/providers/portainer/portainer/latest/docs/resources/kubernetes_delete_object) | resource |

## Inputs

| Name | Description | Type | Default | Required |
|------|-------------|------|---------|:--------:|
| <a name="input_environment_id"></a> [environment\_id](#input\_environment\_id) | ID of the Portainer environment (Kubernetes endpoint). | `number` | n/a | yes |
| <a name="input_names"></a> [names](#input\_names) | List of resource names to delete. | `list(string)` | n/a | yes |
| <a name="input_namespace"></a> [namespace](#input\_namespace) | Kubernetes namespace where the resources to be deleted reside. | `string` | n/a | yes |
| <a name="input_portainer_api_key"></a> [portainer\_api\_key](#input\_portainer\_api\_key) | Default Portainer Admin API Key | `string` | n/a | yes |
| <a name="input_portainer_url"></a> [portainer\_url](#input\_portainer\_url) | Default Portainer URL | `string` | n/a | yes |
| <a name="input_resource_type"></a> [resource\_type](#input\_resource\_type) | Type of resource to delete (e.g. services, ingresses, jobs, cron\_jobs, roles, role\_bindings, service\_accounts). | `string` | n/a | yes |
<!-- END_TF_DOCS -->