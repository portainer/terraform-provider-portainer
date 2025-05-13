<!-- BEGIN_TF_DOCS -->


## Providers

| Name | Version |
|------|---------|
| <a name="provider_portainer"></a> [portainer](#provider\_portainer) | n/a |

## Resources

| Name | Type |
|------|------|
| [portainer_cloud_provider_provision.do_cluster](https://registry.terraform.io/providers/portainer/portainer/latest/docs/resources/cloud_provider_provision) | resource |

## Inputs

| Name | Description | Type | Default | Required |
|------|-------------|------|---------|:--------:|
| <a name="input_cloud_provider"></a> [cloud\_provider](#input\_cloud\_provider) | Cloud provider to use for provisioning (e.g., digitalocean, civo, linode, amazon, azure, gke) | `string` | `"digitalocean"` | no |
| <a name="input_do_credential_id"></a> [do\_credential\_id](#input\_do\_credential\_id) | n/a | `number` | `1` | no |
| <a name="input_do_group_id"></a> [do\_group\_id](#input\_do\_group\_id) | n/a | `number` | `1` | no |
| <a name="input_do_kubernetes_version"></a> [do\_kubernetes\_version](#input\_do\_kubernetes\_version) | n/a | `string` | `"1.25.0"` | no |
| <a name="input_do_name"></a> [do\_name](#input\_do\_name) | n/a | `string` | `"do-dev-cluster"` | no |
| <a name="input_do_network_id"></a> [do\_network\_id](#input\_do\_network\_id) | n/a | `string` | `"1234-abcd"` | no |
| <a name="input_do_node_count"></a> [do\_node\_count](#input\_do\_node\_count) | n/a | `number` | `3` | no |
| <a name="input_do_node_size"></a> [do\_node\_size](#input\_do\_node\_size) | n/a | `string` | `"s-2vcpu-4gb"` | no |
| <a name="input_do_region"></a> [do\_region](#input\_do\_region) | n/a | `string` | `"nyc1"` | no |
| <a name="input_do_stack_name"></a> [do\_stack\_name](#input\_do\_stack\_name) | n/a | `string` | `"dev"` | no |
| <a name="input_do_tag_ids"></a> [do\_tag\_ids](#input\_do\_tag\_ids) | n/a | `list(number)` | <pre>[<br/>  1<br/>]</pre> | no |
| <a name="input_portainer_api_key"></a> [portainer\_api\_key](#input\_portainer\_api\_key) | Default Portainer Admin API Key | `string` | n/a | yes |
| <a name="input_portainer_url"></a> [portainer\_url](#input\_portainer\_url) | Default Portainer URL | `string` | n/a | yes |
<!-- END_TF_DOCS -->