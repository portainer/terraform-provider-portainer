<!-- BEGIN_TF_DOCS -->


## Providers

| Name | Version |
|------|---------|
| <a name="provider_portainer"></a> [portainer](#provider\_portainer) | 1.13.2 |

## Resources

| Name | Type |
|------|------|
| [portainer_endpoint_settings.test](https://registry.terraform.io/providers/portainer/portainer/latest/docs/resources/endpoint_settings) | resource |

## Inputs

| Name | Description | Type | Default | Required |
|------|-------------|------|---------|:--------:|
| <a name="input_allow_bind_mounts"></a> [allow\_bind\_mounts](#input\_allow\_bind\_mounts) | Allow bind mounts for regular users | `bool` | n/a | yes |
| <a name="input_allow_container_capabilities"></a> [allow\_container\_capabilities](#input\_allow\_container\_capabilities) | Allow container capabilities for regular users | `bool` | n/a | yes |
| <a name="input_allow_device_mapping"></a> [allow\_device\_mapping](#input\_allow\_device\_mapping) | Allow device mapping for regular users | `bool` | n/a | yes |
| <a name="input_allow_host_namespace"></a> [allow\_host\_namespace](#input\_allow\_host\_namespace) | Allow host namespace for regular users | `bool` | n/a | yes |
| <a name="input_allow_privileged_mode"></a> [allow\_privileged\_mode](#input\_allow\_privileged\_mode) | Allow privileged mode for regular users | `bool` | n/a | yes |
| <a name="input_allow_stack_management"></a> [allow\_stack\_management](#input\_allow\_stack\_management) | Allow stack management for regular users | `bool` | n/a | yes |
| <a name="input_allow_sysctl_setting"></a> [allow\_sysctl\_setting](#input\_allow\_sysctl\_setting) | Allow sysctl setting for regular users | `bool` | n/a | yes |
| <a name="input_allow_volume_browser"></a> [allow\_volume\_browser](#input\_allow\_volume\_browser) | Allow volume browser for regular users | `bool` | n/a | yes |
| <a name="input_enable_gpu_management"></a> [enable\_gpu\_management](#input\_enable\_gpu\_management) | Enable GPU management | `bool` | n/a | yes |
| <a name="input_enable_host_management"></a> [enable\_host\_management](#input\_enable\_host\_management) | Enable host management features | `bool` | n/a | yes |
| <a name="input_endpoint_id"></a> [endpoint\_id](#input\_endpoint\_id) | ID of the Portainer endpoint | `number` | n/a | yes |
| <a name="input_gpus"></a> [gpus](#input\_gpus) | List of GPU settings (name + value) | <pre>list(object({<br/>    name  = string<br/>    value = string<br/>  }))</pre> | n/a | yes |
| <a name="input_portainer_api_key"></a> [portainer\_api\_key](#input\_portainer\_api\_key) | Default Portainer Admin API Key | `string` | n/a | yes |
| <a name="input_portainer_url"></a> [portainer\_url](#input\_portainer\_url) | Default Portainer URL | `string` | n/a | yes |
<!-- END_TF_DOCS -->