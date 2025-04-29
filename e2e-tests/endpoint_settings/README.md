<!-- BEGIN_TF_DOCS -->


## Providers

| Name | Version |
|------|---------|
| <a name="provider_portainer"></a> [portainer](#provider\_portainer) | n/a |

## Resources

| Name | Type |
|------|------|
| [portainer_endpoint_settings.test](https://registry.terraform.io/providers/portainer/portainer/latest/docs/resources/endpoint_settings) | resource |

## Inputs

| Name | Description | Type | Default | Required |
|------|-------------|------|---------|:--------:|
| <a name="input_allow_bind_mounts"></a> [allow\_bind\_mounts](#input\_allow\_bind\_mounts) | Allow bind mounts for regular users | `bool` | `true` | no |
| <a name="input_allow_container_capabilities"></a> [allow\_container\_capabilities](#input\_allow\_container\_capabilities) | Allow container capabilities for regular users | `bool` | `true` | no |
| <a name="input_allow_device_mapping"></a> [allow\_device\_mapping](#input\_allow\_device\_mapping) | Allow device mapping for regular users | `bool` | `true` | no |
| <a name="input_allow_host_namespace"></a> [allow\_host\_namespace](#input\_allow\_host\_namespace) | Allow host namespace for regular users | `bool` | `true` | no |
| <a name="input_allow_privileged_mode"></a> [allow\_privileged\_mode](#input\_allow\_privileged\_mode) | Allow privileged mode for regular users | `bool` | `false` | no |
| <a name="input_allow_stack_management"></a> [allow\_stack\_management](#input\_allow\_stack\_management) | Allow stack management for regular users | `bool` | `true` | no |
| <a name="input_allow_sysctl_setting"></a> [allow\_sysctl\_setting](#input\_allow\_sysctl\_setting) | Allow sysctl setting for regular users | `bool` | `true` | no |
| <a name="input_allow_volume_browser"></a> [allow\_volume\_browser](#input\_allow\_volume\_browser) | Allow volume browser for regular users | `bool` | `true` | no |
| <a name="input_enable_gpu_management"></a> [enable\_gpu\_management](#input\_enable\_gpu\_management) | Enable GPU management | `bool` | `false` | no |
| <a name="input_enable_host_management"></a> [enable\_host\_management](#input\_enable\_host\_management) | Enable host management features | `bool` | `true` | no |
| <a name="input_endpoint_id"></a> [endpoint\_id](#input\_endpoint\_id) | ID of the Portainer endpoint | `number` | `3` | no |
| <a name="input_gpus"></a> [gpus](#input\_gpus) | List of GPU settings (name + value) | <pre>list(object({<br/>    name  = string<br/>    value = string<br/>  }))</pre> | <pre>[<br/>  {<br/>    "name": "nvidia",<br/>    "value": "gpu0"<br/>  }<br/>]</pre> | no |
| <a name="input_portainer_api_key"></a> [portainer\_api\_key](#input\_portainer\_api\_key) | Default Portainer Admin API Key | `string` | `"ptr_xrP7XWqfZEOoaCJRu5c8qKaWuDtVc2Zb07Q5g22YpS8="` | no |
| <a name="input_portainer_skip_ssl_verify"></a> [portainer\_skip\_ssl\_verify](#input\_portainer\_skip\_ssl\_verify) | Set to true to skip TLS certificate verification (useful for self-signed certs) | `bool` | `true` | no |
| <a name="input_portainer_url"></a> [portainer\_url](#input\_portainer\_url) | Default Portainer URL | `string` | `"https://localhost:9443"` | no |
<!-- END_TF_DOCS -->