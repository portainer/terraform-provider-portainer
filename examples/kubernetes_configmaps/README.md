<!-- BEGIN_TF_DOCS -->


## Providers

| Name | Version |
|------|---------|
| <a name="provider_portainer"></a> [portainer](#provider\_portainer) | n/a |

## Resources

| Name | Type |
|------|------|
| [portainer_kubernetes_configmaps.example](https://registry.terraform.io/providers/portainer/portainer/latest/docs/resources/kubernetes_configmaps) | resource |

## Inputs

| Name | Description | Type | Default | Required |
|------|-------------|------|---------|:--------:|
| <a name="input_endpoint_id"></a> [endpoint\_id](#input\_endpoint\_id) | ID of the Portainer environment (Kubernetes cluster) | `number` | `4` | no |
| <a name="input_manifest_file"></a> [manifest\_file](#input\_manifest\_file) | Path to the Kubernetes configmaps manifest (YAML or JSON) | `string` | `"configmaps.yaml"` | no |
| <a name="input_namespace"></a> [namespace](#input\_namespace) | Kubernetes namespace where the configmaps will be deployed | `string` | `"default"` | no |
| <a name="input_portainer_api_key"></a> [portainer\_api\_key](#input\_portainer\_api\_key) | Default Portainer Admin API Key | `string` | n/a | yes |
| <a name="input_portainer_url"></a> [portainer\_url](#input\_portainer\_url) | Default Portainer URL | `string` | n/a | yes |
<!-- END_TF_DOCS -->