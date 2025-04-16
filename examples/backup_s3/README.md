<!-- BEGIN_TF_DOCS -->


## Providers

| Name | Version |
|------|---------|
| <a name="provider_portainer"></a> [portainer](#provider\_portainer) | n/a |

## Resources

| Name | Type |
|------|------|
| [portainer_backup_s3.your_s3_backup](https://registry.terraform.io/providers/portainer/portainer/latest/docs/resources/backup_s3) | resource |

## Inputs

| Name | Description | Type | Default | Required |
|------|-------------|------|---------|:--------:|
| <a name="input_backup_cron_rule"></a> [backup\_cron\_rule](#input\_backup\_cron\_rule) | Cron rule for scheduling the backup (e.g., '@daily') | `string` | `"@daily"` | no |
| <a name="input_backup_password"></a> [backup\_password](#input\_backup\_password) | Password used to encrypt the Portainer backup archive | `string` | n/a | yes |
| <a name="input_portainer_api_key"></a> [portainer\_api\_key](#input\_portainer\_api\_key) | Default Portainer Admin API Key | `string` | n/a | yes |
| <a name="input_portainer_url"></a> [portainer\_url](#input\_portainer\_url) | Default Portainer URL | `string` | n/a | yes |
| <a name="input_s3_access_key"></a> [s3\_access\_key](#input\_s3\_access\_key) | AWS or compatible S3 Access Key | `string` | n/a | yes |
| <a name="input_s3_bucket"></a> [s3\_bucket](#input\_s3\_bucket) | S3 bucket name where backups will be stored | `string` | n/a | yes |
| <a name="input_s3_endpoint"></a> [s3\_endpoint](#input\_s3\_endpoint) | S3-compatible endpoint URL | `string` | n/a | yes |
| <a name="input_s3_region"></a> [s3\_region](#input\_s3\_region) | Region for S3 bucket (e.g., eu-central-1) | `string` | `"eu-central-1"` | no |
| <a name="input_s3_secret_key"></a> [s3\_secret\_key](#input\_s3\_secret\_key) | AWS or compatible S3 Secret Access Key | `string` | n/a | yes |
<!-- END_TF_DOCS -->