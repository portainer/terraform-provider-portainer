<!-- BEGIN_TF_DOCS -->


## Providers

| Name | Version |
|------|---------|
| <a name="provider_portainer"></a> [portainer](#provider\_portainer) | 0.1.0 |

## Resources

| Name | Type |
|------|------|
| portainer_docker_node.example | resource |

## Inputs

| Name | Description | Type | Default | Required |
|------|-------------|------|---------|:--------:|
| <a name="input_docker_node_availability"></a> [docker\_node\_availability](#input\_docker\_node\_availability) | Availability of the node (active, pause, drain) | `string` | `"active"` | no |
| <a name="input_docker_node_endpoint_id"></a> [docker\_node\_endpoint\_id](#input\_docker\_node\_endpoint\_id) | ID of the Portainer endpoint (environment) | `number` | `1` | no |
| <a name="input_docker_node_id"></a> [docker\_node\_id](#input\_docker\_node\_id) | ID of the Docker Swarm node | `string` | `"wna048ajhbc1n1t5ispvf6mvg"` | no |
| <a name="input_docker_node_labels"></a> [docker\_node\_labels](#input\_docker\_node\_labels) | Map of node labels | `map(string)` | <pre>{<br/>  "foo": "barrerun"<br/>}</pre> | no |
| <a name="input_docker_node_name"></a> [docker\_node\_name](#input\_docker\_node\_name) | Custom name of the node | `string` | `"node-name"` | no |
| <a name="input_docker_node_role"></a> [docker\_node\_role](#input\_docker\_node\_role) | Role of the node (manager or worker) | `string` | `"manager"` | no |
| <a name="input_docker_node_version"></a> [docker\_node\_version](#input\_docker\_node\_version) | Swarm node version required for update/delete | `number` | `4869` | no |
| <a name="input_portainer_api_key"></a> [portainer\_api\_key](#input\_portainer\_api\_key) | Default Portainer Admin API Key | `string` | n/a | yes |
| <a name="input_portainer_url"></a> [portainer\_url](#input\_portainer\_url) | Default Portainer URL | `string` | n/a | yes |
<!-- END_TF_DOCS -->