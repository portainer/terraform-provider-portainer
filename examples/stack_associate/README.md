<!-- BEGIN_TF_DOCS -->


## Providers

| Name | Version |
|------|---------|
| <a name="provider_portainer"></a> [portainer](#provider\_portainer) | 1.13.2 |

## Resources

| Name | Type |
|------|------|
| [portainer_stack_associate.example](https://registry.terraform.io/providers/portainer/portainer/latest/docs/resources/stack_associate) | resource |

## Inputs

| Name | Description | Type | Default | Required |
|------|-------------|------|---------|:--------:|
| <a name="input_portainer_api_key"></a> [portainer\_api\_key](#input\_portainer\_api\_key) | Default Portainer Admin API Key | `string` | n/a | yes |
| <a name="input_portainer_url"></a> [portainer\_url](#input\_portainer\_url) | Default Portainer URL | `string` | n/a | yes |
| <a name="input_stack_associate_endpoint_id"></a> [stack\_associate\_endpoint\_id](#input\_stack\_associate\_endpoint\_id) | ID of the environment (endpoint) to associate the stack with | `number` | `1` | no |
| <a name="input_stack_associate_orphaned_running"></a> [stack\_associate\_orphaned\_running](#input\_stack\_associate\_orphaned\_running) | Whether the stack is an orphaned running stack | `bool` | `true` | no |
| <a name="input_stack_associate_stack_id"></a> [stack\_associate\_stack\_id](#input\_stack\_associate\_stack\_id) | ID of the orphaned stack to associate | `number` | `12` | no |
| <a name="input_stack_associate_swarm_id"></a> [stack\_associate\_swarm\_id](#input\_stack\_associate\_swarm\_id) | ID of the Swarm cluster where the stack should be associated | `string` | `"jpofkc0i9uo9wtx1zesuk649w"` | no |
<!-- END_TF_DOCS -->