<!-- BEGIN_TF_DOCS -->


## Providers

| Name | Version |
|------|---------|
| <a name="provider_portainer"></a> [portainer](#provider\_portainer) | 1.13.1 |

## Resources

| Name | Type |
|------|------|
| [portainer_docker_network.test_bridge](https://registry.terraform.io/providers/portainer/portainer/latest/docs/resources/docker_network) | resource |

## Inputs

| Name | Description | Type | Default | Required |
|------|-------------|------|---------|:--------:|
| <a name="input_endpoint_id"></a> [endpoint\_id](#input\_endpoint\_id) | ID of the environment where the network will be created | `number` | `3` | no |
| <a name="input_network_attachable"></a> [network\_attachable](#input\_network\_attachable) | Whether containers can be attached manually | `bool` | `false` | no |
| <a name="input_network_config_from"></a> [network\_config\_from](#input\_network\_config\_from) | Name of another config-only network to inherit from | `string` | `""` | no |
| <a name="input_network_config_only"></a> [network\_config\_only](#input\_network\_config\_only) | If this network is only configuration | `bool` | `false` | no |
| <a name="input_network_driver"></a> [network\_driver](#input\_network\_driver) | Network driver (bridge, overlay, macvlan, etc.) | `string` | `"bridge"` | no |
| <a name="input_network_enable_ipv4"></a> [network\_enable\_ipv4](#input\_network\_enable\_ipv4) | Enable IPv4 networking | `bool` | `true` | no |
| <a name="input_network_enable_ipv6"></a> [network\_enable\_ipv6](#input\_network\_enable\_ipv6) | Enable IPv6 networking | `bool` | `false` | no |
| <a name="input_network_ingress"></a> [network\_ingress](#input\_network\_ingress) | Whether it's an ingress network | `bool` | `false` | no |
| <a name="input_network_internal"></a> [network\_internal](#input\_network\_internal) | Whether the network is internal | `bool` | `false` | no |
| <a name="input_network_labels"></a> [network\_labels](#input\_network\_labels) | Labels to apply to the network | `map(string)` | <pre>{<br/>  "env": "test",<br/>  "purpose": "terraform"<br/>}</pre> | no |
| <a name="input_network_name"></a> [network\_name](#input\_network\_name) | Name of the Docker network | `string` | `"your-network"` | no |
| <a name="input_network_options"></a> [network\_options](#input\_network\_options) | Driver-specific options | `map(string)` | <pre>{<br/>  "com.docker.network.bridge.enable_icc": "true",<br/>  "com.docker.network.bridge.enable_ip_masquerade": "true"<br/>}</pre> | no |
| <a name="input_portainer_api_key"></a> [portainer\_api\_key](#input\_portainer\_api\_key) | Default Portainer Admin API Key | `string` | `"ptr_xrP7XWqfZEOoaCJRu5c8qKaWuDtVc2Zb07Q5g22YpS8="` | no |
| <a name="input_portainer_skip_ssl_verify"></a> [portainer\_skip\_ssl\_verify](#input\_portainer\_skip\_ssl\_verify) | Set to true to skip TLS certificate verification (useful for self-signed certs) | `bool` | `true` | no |
| <a name="input_portainer_url"></a> [portainer\_url](#input\_portainer\_url) | Default Portainer URL | `string` | `"https://localhost:9443"` | no |
<!-- END_TF_DOCS -->