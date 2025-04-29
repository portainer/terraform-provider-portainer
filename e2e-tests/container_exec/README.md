<!-- BEGIN_TF_DOCS -->


## Providers

| Name | Version |
|------|---------|
| <a name="provider_portainer"></a> [portainer](#provider\_portainer) | n/a |

## Resources

| Name | Type |
|------|------|
| [portainer_container_exec.standalone](https://registry.terraform.io/providers/portainer/portainer/latest/docs/resources/container_exec) | resource |

## Inputs

| Name | Description | Type | Default | Required |
|------|-------------|------|---------|:--------:|
| <a name="input_portainer_api_key"></a> [portainer\_api\_key](#input\_portainer\_api\_key) | Default Portainer Admin API Key | `string` | `"ptr_xrP7XWqfZEOoaCJRu5c8qKaWuDtVc2Zb07Q5g22YpS8="` | no |
| <a name="input_portainer_exec_command"></a> [portainer\_exec\_command](#input\_portainer\_exec\_command) | Command to execute inside the container | `string` | `"ls -alh"` | no |
| <a name="input_portainer_exec_endpoint_id"></a> [portainer\_exec\_endpoint\_id](#input\_portainer\_exec\_endpoint\_id) | Portainer endpoint ID (standalone or swarm) | `number` | `3` | no |
| <a name="input_portainer_exec_service_name"></a> [portainer\_exec\_service\_name](#input\_portainer\_exec\_service\_name) | Name of the container (standalone) or service (swarm) | `string` | `"nginx-standalone-string-web-1"` | no |
| <a name="input_portainer_exec_user"></a> [portainer\_exec\_user](#input\_portainer\_exec\_user) | User to run the command as (e.g. root, uid) | `string` | `"root"` | no |
| <a name="input_portainer_skip_ssl_verify"></a> [portainer\_skip\_ssl\_verify](#input\_portainer\_skip\_ssl\_verify) | Set to true to skip TLS certificate verification (useful for self-signed certs) | `bool` | `true` | no |
| <a name="input_portainer_url"></a> [portainer\_url](#input\_portainer\_url) | Default Portainer URL | `string` | `"https://localhost:9443"` | no |

## Outputs

| Name | Description |
|------|-------------|
| <a name="output_container_exec_output"></a> [container\_exec\_output](#output\_container\_exec\_output) | n/a |
<!-- END_TF_DOCS -->