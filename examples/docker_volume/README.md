<!-- BEGIN_TF_DOCS -->


## Providers

| Name | Version |
|------|---------|
| <a name="provider_portainer"></a> [portainer](#provider\_portainer) | n/a |

## Resources

| Name | Type |
|------|------|
| [portainer_docker_volume.example](https://registry.terraform.io/providers/portainer/portainer/latest/docs/resources/docker_volume) | resource |

## Inputs

| Name | Description | Type | Default | Required |
|------|-------------|------|---------|:--------:|
| <a name="input_endpoint_id"></a> [endpoint\_id](#input\_endpoint\_id) | ID of the Portainer endpoint/environment | `number` | n/a | yes |
| <a name="input_portainer_api_key"></a> [portainer\_api\_key](#input\_portainer\_api\_key) | Default Portainer Admin API Key | `string` | n/a | yes |
| <a name="input_portainer_url"></a> [portainer\_url](#input\_portainer\_url) | Default Portainer URL | `string` | n/a | yes |
| <a name="input_volume_driver"></a> [volume\_driver](#input\_volume\_driver) | Docker volume driver to use | `string` | `"local"` | no |
| <a name="input_volume_driver_opts"></a> [volume\_driver\_opts](#input\_volume\_driver\_opts) | Driver-specific options | `map(string)` | <pre>{<br/>  "device": "tmpfs",<br/>  "o": "size=100m,uid=1000",<br/>  "type": "tmpfs"<br/>}</pre> | no |
| <a name="input_volume_labels"></a> [volume\_labels](#input\_volume\_labels) | Labels to apply to the volume | `map(string)` | <pre>{<br/>  "env": "test",<br/>  "managed": "terraform"<br/>}</pre> | no |
| <a name="input_volume_name"></a> [volume\_name](#input\_volume\_name) | Name of the Docker volume | `string` | n/a | yes |
<!-- END_TF_DOCS -->