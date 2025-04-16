<!-- BEGIN_TF_DOCS -->


## Providers

| Name | Version |
|------|---------|
| <a name="provider_portainer"></a> [portainer](#provider\_portainer) | n/a |

## Resources

| Name | Type |
|------|------|
| [portainer_edge_job.string_job](https://registry.terraform.io/providers/portainer/portainer/latest/docs/resources/edge_job) | resource |

## Inputs

| Name | Description | Type | Default | Required |
|------|-------------|------|---------|:--------:|
| <a name="input_edge_job_cron"></a> [edge\_job\_cron](#input\_edge\_job\_cron) | Cron expression for edge job scheduling | `string` | n/a | yes |
| <a name="input_edge_job_edge_groups"></a> [edge\_job\_edge\_groups](#input\_edge\_job\_edge\_groups) | List of edge group IDs | `list(number)` | n/a | yes |
| <a name="input_edge_job_endpoints"></a> [edge\_job\_endpoints](#input\_edge\_job\_endpoints) | List of environment (endpoint) IDs | `list(number)` | n/a | yes |
| <a name="input_edge_job_file_content"></a> [edge\_job\_file\_content](#input\_edge\_job\_file\_content) | Script content to run on edge agents | `string` | n/a | yes |
| <a name="input_edge_job_name"></a> [edge\_job\_name](#input\_edge\_job\_name) | Name of the edge job | `string` | n/a | yes |
| <a name="input_edge_job_recurring"></a> [edge\_job\_recurring](#input\_edge\_job\_recurring) | Whether the edge job should be recurring | `bool` | n/a | yes |
| <a name="input_portainer_api_key"></a> [portainer\_api\_key](#input\_portainer\_api\_key) | Default Portainer Admin API Key | `string` | n/a | yes |
| <a name="input_portainer_url"></a> [portainer\_url](#input\_portainer\_url) | Default Portainer URL | `string` | n/a | yes |
<!-- END_TF_DOCS -->