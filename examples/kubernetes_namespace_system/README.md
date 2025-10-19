<!-- BEGIN_TF_DOCS -->


## Providers

| Name | Version |
|------|---------|
| <a name="provider_portainer"></a> [portainer](#provider\_portainer) | 1.13.2 |

## Resources

| Name | Type |
|------|------|
| [portainer_kubernetes_namespace_system.example](https://registry.terraform.io/providers/portainer/portainer/latest/docs/resources/kubernetes_namespace_system) | resource |

## Inputs

| Name | Description | Type | Default | Required |
|------|-------------|------|---------|:--------:|
| <a name="input_environment_id"></a> [environment\_id](#input\_environment\_id) | Portainer environment (Kubernetes endpoint) ID | `number` | `4` | no |
| <a name="input_namespace"></a> [namespace](#input\_namespace) | Kubernetes namespace to toggle system state for | `string` | `"default"` | no |
| <a name="input_portainer_api_key"></a> [portainer\_api\_key](#input\_portainer\_api\_key) | Default Portainer Admin API Key | `string` | n/a | yes |
| <a name="input_portainer_url"></a> [portainer\_url](#input\_portainer\_url) | Default Portainer URL | `string` | n/a | yes |
| <a name="input_system"></a> [system](#input\_system) | Whether the namespace should be marked as a system namespace | `bool` | `true` | no |
<!-- END_TF_DOCS -->