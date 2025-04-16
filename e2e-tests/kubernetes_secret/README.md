<!-- BEGIN_TF_DOCS -->


## Providers

| Name | Version |
|------|---------|
| <a name="provider_portainer"></a> [portainer](#provider\_portainer) | n/a |

## Resources

| Name | Type |
|------|------|
| [portainer_kubernetes_secret.example](https://registry.terraform.io/providers/portainer/portainer/latest/docs/resources/kubernetes_secret) | resource |

## Inputs

| Name | Description | Type | Default | Required |
|------|-------------|------|---------|:--------:|
| <a name="input_endpoint_id"></a> [endpoint\_id](#input\_endpoint\_id) | ID of the Portainer environment (Kubernetes cluster) | `number` | `4` | no |
| <a name="input_manifest_file"></a> [manifest\_file](#input\_manifest\_file) | Path to the Kubernetes secret manifest (YAML or JSON) | `string` | `"secret.yaml"` | no |
| <a name="input_namespace"></a> [namespace](#input\_namespace) | Kubernetes namespace where the secret will be deployed | `string` | `"default"` | no |
| <a name="input_portainer_api_key"></a> [portainer\_api\_key](#input\_portainer\_api\_key) | Default Portainer Admin API Key | `string` | `"ptr_xrP7XWqfZEOoaCJRu5c8qKaWuDtVc2Zb07Q5g22YpS8="` | no |
| <a name="input_portainer_url"></a> [portainer\_url](#input\_portainer\_url) | Default Portainer URL | `string` | `"http://localhost:9000"` | no |
<!-- END_TF_DOCS -->