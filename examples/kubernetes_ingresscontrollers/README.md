<!-- BEGIN_TF_DOCS -->


## Providers

| Name | Version |
|------|---------|
| <a name="provider_portainer"></a> [portainer](#provider\_portainer) | n/a |

## Resources

| Name | Type |
|------|------|
| [portainer_kubernetes_ingresscontrollers.test](https://registry.terraform.io/providers/portainer/portainer/latest/docs/resources/kubernetes_ingresscontrollers) | resource |

## Inputs

| Name | Description | Type | Default | Required |
|------|-------------|------|---------|:--------:|
| <a name="input_controllers"></a> [controllers](#input\_controllers) | List of ingress controller configurations. | <pre>list(object({<br/>    name         = string<br/>    class_name   = string<br/>    type         = string<br/>    availability = bool<br/>    used         = bool<br/>    new          = bool<br/>  }))</pre> | n/a | yes |
| <a name="input_environment_id"></a> [environment\_id](#input\_environment\_id) | ID of the Kubernetes environment (endpoint). | `number` | n/a | yes |
| <a name="input_portainer_api_key"></a> [portainer\_api\_key](#input\_portainer\_api\_key) | Default Portainer Admin API Key | `string` | n/a | yes |
| <a name="input_portainer_url"></a> [portainer\_url](#input\_portainer\_url) | Default Portainer URL | `string` | n/a | yes |
<!-- END_TF_DOCS -->