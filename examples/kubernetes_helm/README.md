<!-- BEGIN_TF_DOCS -->


## Providers

| Name | Version |
|------|---------|
| <a name="provider_portainer"></a> [portainer](#provider\_portainer) | 1.13.2 |

## Resources

| Name | Type |
|------|------|
| [portainer_kubernetes_helm.example](https://registry.terraform.io/providers/portainer/portainer/latest/docs/resources/kubernetes_helm) | resource |

## Inputs

| Name | Description | Type | Default | Required |
|------|-------------|------|---------|:--------:|
| <a name="input_environment_id"></a> [environment\_id](#input\_environment\_id) | The ID of the Kubernetes environment (endpoint) in Portainer where the Helm chart will be deployed | `number` | n/a | yes |
| <a name="input_helm_chart"></a> [helm\_chart](#input\_helm\_chart) | The name of the Helm chart, e.g., nginx or redis | `string` | n/a | yes |
| <a name="input_helm_namespace"></a> [helm\_namespace](#input\_helm\_namespace) | The Kubernetes namespace where the Helm chart should be deployed | `string` | `"default"` | no |
| <a name="input_helm_release_name"></a> [helm\_release\_name](#input\_helm\_release\_name) | The name of the Helm release | `string` | n/a | yes |
| <a name="input_helm_repo"></a> [helm\_repo](#input\_helm\_repo) | The URL of the Helm chart repository | `string` | n/a | yes |
| <a name="input_helm_values"></a> [helm\_values](#input\_helm\_values) | Optional Helm chart values provided as a raw YAML string | `string` | `""` | no |
| <a name="input_portainer_api_key"></a> [portainer\_api\_key](#input\_portainer\_api\_key) | Default Portainer Admin API Key | `string` | n/a | yes |
| <a name="input_portainer_url"></a> [portainer\_url](#input\_portainer\_url) | Default Portainer URL | `string` | n/a | yes |
<!-- END_TF_DOCS -->