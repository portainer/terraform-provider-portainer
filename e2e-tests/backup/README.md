<!-- BEGIN_TF_DOCS -->


## Providers

| Name | Version |
|------|---------|
| <a name="provider_portainer"></a> [portainer](#provider\_portainer) | n/a |

## Resources

| Name | Type |
|------|------|
| [portainer_backup.snapshot](https://registry.terraform.io/providers/portainer/portainer/latest/docs/resources/backup) | resource |

## Inputs

| Name | Description | Type | Default | Required |
|------|-------------|------|---------|:--------:|
| <a name="input_portainer_api_key"></a> [portainer\_api\_key](#input\_portainer\_api\_key) | Default Portainer Admin API Key | `string` | `"ptr_xrP7XWqfZEOoaCJRu5c8qKaWuDtVc2Zb07Q5g22YpS8="` | no |
| <a name="input_portainer_backup_output_path"></a> [portainer\_backup\_output\_path](#input\_portainer\_backup\_output\_path) | Path to store the output backup file | `string` | `"backup.tar.gz"` | no |
| <a name="input_portainer_backup_password"></a> [portainer\_backup\_password](#input\_portainer\_backup\_password) | Password used to encrypt the Portainer backup | `string` | `"secure-password-for-backup"` | no |
| <a name="input_portainer_url"></a> [portainer\_url](#input\_portainer\_url) | Default Portainer URL | `string` | `"http://localhost:9000"` | no |
<!-- END_TF_DOCS -->