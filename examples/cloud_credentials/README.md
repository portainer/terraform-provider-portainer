<!-- BEGIN_TF_DOCS -->


## Providers

| Name | Version |
|------|---------|
| <a name="provider_portainer"></a> [portainer](#provider\_portainer) | 1.13.2 |

## Resources

| Name | Type |
|------|------|
| [portainer_cloud_credentials.example](https://registry.terraform.io/providers/portainer/portainer/latest/docs/resources/cloud_credentials) | resource |

## Inputs

| Name | Description | Type | Default | Required |
|------|-------------|------|---------|:--------:|
| <a name="input_cloud_credentials_data"></a> [cloud\_credentials\_data](#input\_cloud\_credentials\_data) | JSON-encoded credentials block for the cloud provider | `string` | `"{\n  \"accessKeyId\": \"your-access-key\",\n  \"secretAccessKey\": \"your-secret-key\",\n  \"region\": \"eu-central-1\"\n}\n"` | no |
| <a name="input_cloud_credentials_name"></a> [cloud\_credentials\_name](#input\_cloud\_credentials\_name) | Name of the cloud credential (e.g., my-aws-creds) | `string` | `"example-aws-creds"` | no |
| <a name="input_cloud_credentials_provider"></a> [cloud\_credentials\_provider](#input\_cloud\_credentials\_provider) | Cloud provider (e.g., aws, digitalocean, civo) | `string` | `"aws"` | no |
| <a name="input_portainer_api_key"></a> [portainer\_api\_key](#input\_portainer\_api\_key) | Default Portainer Admin API Key | `string` | n/a | yes |
| <a name="input_portainer_url"></a> [portainer\_url](#input\_portainer\_url) | Default Portainer URL | `string` | n/a | yes |
<!-- END_TF_DOCS -->