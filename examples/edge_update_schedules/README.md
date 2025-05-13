<!-- BEGIN_TF_DOCS -->


## Providers

| Name | Version |
|------|---------|
| <a name="provider_portainer"></a> [portainer](#provider\_portainer) | n/a |

## Resources

| Name | Type |
|------|------|
| [portainer_edge_update_schedules.example](https://registry.terraform.io/providers/portainer/portainer/latest/docs/resources/edge_update_schedules) | resource |

## Inputs

| Name | Description | Type | Default | Required |
|------|-------------|------|---------|:--------:|
| <a name="input_agent_image"></a> [agent\_image](#input\_agent\_image) | n/a | `string` | `"portainer/agent:2.19.0"` | no |
| <a name="input_edge_group_ids"></a> [edge\_group\_ids](#input\_edge\_group\_ids) | n/a | `list(number)` | <pre>[<br/>  1<br/>]</pre> | no |
| <a name="input_edge_schedule_name"></a> [edge\_schedule\_name](#input\_edge\_schedule\_name) | n/a | `string` | `"scheduled-edge-update"` | no |
| <a name="input_portainer_api_key"></a> [portainer\_api\_key](#input\_portainer\_api\_key) | Default Portainer Admin API Key | `string` | n/a | yes |
| <a name="input_portainer_url"></a> [portainer\_url](#input\_portainer\_url) | Default Portainer URL | `string` | n/a | yes |
| <a name="input_registry_id"></a> [registry\_id](#input\_registry\_id) | n/a | `number` | `1` | no |
| <a name="input_scheduled_time"></a> [scheduled\_time](#input\_scheduled\_time) | RFC3339 formatted time for update (UTC) | `string` | `"2025-05-10T10:00:00Z"` | no |
| <a name="input_update_type"></a> [update\_type](#input\_update\_type) | 0 = update, 1 = rollback | `number` | `0` | no |
| <a name="input_updater_image"></a> [updater\_image](#input\_updater\_image) | n/a | `string` | `"portainer/portainer-updater:2.19.0"` | no |
<!-- END_TF_DOCS -->