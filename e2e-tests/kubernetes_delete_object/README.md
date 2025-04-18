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
| <a name="input_environment_id"></a> [environment\_id](#input\_environment\_id) | ID of the Portainer environment (Kubernetes endpoint). | `number` | `4` | no |
| <a name="input_names"></a> [names](#input\_names) | List of resource names to delete. | `list(string)` | <pre>[<br/>  "demo-role"<br/>]</pre> | no |
| <a name="input_namespace"></a> [namespace](#input\_namespace) | Kubernetes namespace where the resources to be deleted reside. | `string` | `"default"` | no |
| <a name="input_portainer_api_key"></a> [portainer\_api\_key](#input\_portainer\_api\_key) | Default Portainer Admin API Key | `string` | `"ptr_xrP7XWqfZEOoaCJRu5c8qKaWuDtVc2Zb07Q5g22YpS8="` | no |
| <a name="input_portainer_url"></a> [portainer\_url](#input\_portainer\_url) | Default Portainer URL | `string` | `"http://localhost:9000"` | no |
| <a name="input_resource_type"></a> [resource\_type](#input\_resource\_type) | Type of resource to delete (e.g. services, ingresses, jobs, cron\_jobs, roles, role\_bindings, service\_accounts). | `string` | `"roles"` | no |
<!-- END_TF_DOCS -->