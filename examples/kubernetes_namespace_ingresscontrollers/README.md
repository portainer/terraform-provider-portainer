<!-- BEGIN_TF_DOCS -->


## Providers

| Name | Version |
|------|---------|
| <a name="provider_portainer"></a> [portainer](#provider\_portainer) | n/a |

## Resources

| Name | Type |
|------|------|
| [portainer_kubernetes_namespace_ingresscontrollers.test](https://registry.terraform.io/providers/portainer/portainer/latest/docs/resources/kubernetes_namespace_ingresscontrollers) | resource |

## Inputs

| Name | Description | Type | Default | Required |
|------|-------------|------|---------|:--------:|
| <a name="input_environment_id"></a> [environment\_id](#input\_environment\_id) | The ID of the Kubernetes environment (endpoint) in Portainer. | `number` | n/a | yes |
| <a name="input_ingress_controller"></a> [ingress\_controller](#input\_ingress\_controller) | Configuration for the Kubernetes ingress controller. | <pre>object({<br/>    name         = string<br/>    class_name   = string<br/>    type         = string<br/>    availability = bool<br/>    used         = bool<br/>    new          = bool<br/>  })</pre> | n/a | yes |
| <a name="input_namespace"></a> [namespace](#input\_namespace) | The name of the Kubernetes namespace where the ingress controllers should be applied. | `string` | n/a | yes |
| <a name="input_portainer_api_key"></a> [portainer\_api\_key](#input\_portainer\_api\_key) | Default Portainer Admin API Key | `string` | n/a | yes |
| <a name="input_portainer_url"></a> [portainer\_url](#input\_portainer\_url) | Default Portainer URL | `string` | n/a | yes |
<!-- END_TF_DOCS -->