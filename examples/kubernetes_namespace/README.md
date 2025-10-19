<!-- BEGIN_TF_DOCS -->


## Providers

| Name | Version |
|------|---------|
| <a name="provider_portainer"></a> [portainer](#provider\_portainer) | 1.13.2 |

## Resources

| Name | Type |
|------|------|
| [portainer_kubernetes_namespace.test](https://registry.terraform.io/providers/portainer/portainer/latest/docs/resources/kubernetes_namespace) | resource |

## Inputs

| Name | Description | Type | Default | Required |
|------|-------------|------|---------|:--------:|
| <a name="input_environment_id"></a> [environment\_id](#input\_environment\_id) | ID of the Portainer environment (Kubernetes endpoint). | `number` | n/a | yes |
| <a name="input_namespace_annotations"></a> [namespace\_annotations](#input\_namespace\_annotations) | Map of annotations to apply to the namespace. | `map(string)` | <pre>{<br/>  "env": "test",<br/>  "owner": "terraform"<br/>}</pre> | no |
| <a name="input_namespace_name"></a> [namespace\_name](#input\_namespace\_name) | Name of the Kubernetes namespace to create. | `string` | n/a | yes |
| <a name="input_namespace_owner"></a> [namespace\_owner](#input\_namespace\_owner) | Owner label for the namespace. | `string` | `""` | no |
| <a name="input_namespace_resource_quota"></a> [namespace\_resource\_quota](#input\_namespace\_resource\_quota) | CPU and memory resource quota for the namespace. | <pre>object({<br/>    cpu    = string<br/>    memory = string<br/>  })</pre> | <pre>{<br/>  "cpu": "800m",<br/>  "memory": "129Mi"<br/>}</pre> | no |
| <a name="input_portainer_api_key"></a> [portainer\_api\_key](#input\_portainer\_api\_key) | Default Portainer Admin API Key | `string` | n/a | yes |
| <a name="input_portainer_url"></a> [portainer\_url](#input\_portainer\_url) | Default Portainer URL | `string` | n/a | yes |
<!-- END_TF_DOCS -->