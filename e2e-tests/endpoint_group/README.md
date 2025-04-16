<!-- BEGIN_TF_DOCS -->


## Providers

| Name | Version |
|------|---------|
| <a name="provider_portainer"></a> [portainer](#provider\_portainer) | n/a |

## Resources

| Name | Type |
|------|------|
| [portainer_endpoint_group.your-group](https://registry.terraform.io/providers/portainer/portainer/latest/docs/resources/endpoint_group) | resource |

## Inputs

| Name | Description | Type | Default | Required |
|------|-------------|------|---------|:--------:|
| <a name="input_portainer_api_key"></a> [portainer\_api\_key](#input\_portainer\_api\_key) | Default Portainer Admin API Key | `string` | `"ptr_xrP7XWqfZEOoaCJRu5c8qKaWuDtVc2Zb07Q5g22YpS8="` | no |
| <a name="input_portainer_endpoint_group_description"></a> [portainer\_endpoint\_group\_description](#input\_portainer\_endpoint\_group\_description) | Description of the group | `string` | `"Description for your group"` | no |
| <a name="input_portainer_endpoint_group_name"></a> [portainer\_endpoint\_group\_name](#input\_portainer\_endpoint\_group\_name) | Name of the Portainer endpoint group | `string` | `"your-group"` | no |
| <a name="input_portainer_url"></a> [portainer\_url](#input\_portainer\_url) | Default Portainer URL | `string` | `"http://localhost:9000"` | no |
<!-- END_TF_DOCS -->