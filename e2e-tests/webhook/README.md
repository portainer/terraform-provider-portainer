<!-- BEGIN_TF_DOCS -->


## Providers

| Name | Version |
|------|---------|
| <a name="provider_portainer"></a> [portainer](#provider\_portainer) | n/a |

## Resources

| Name | Type |
|------|------|
| [portainer_webhook.example](https://registry.terraform.io/providers/portainer/portainer/latest/docs/resources/webhook) | resource |

## Inputs

| Name | Description | Type | Default | Required |
|------|-------------|------|---------|:--------:|
| <a name="input_endpoint_id"></a> [endpoint\_id](#input\_endpoint\_id) | Portainer environment/endpoint ID | `number` | `1` | no |
| <a name="input_portainer_api_key"></a> [portainer\_api\_key](#input\_portainer\_api\_key) | Default Portainer Admin API Key | `string` | `"ptr_xrP7XWqfZEOoaCJRu5c8qKaWuDtVc2Zb07Q5g22YpS8="` | no |
| <a name="input_portainer_skip_ssl_verify"></a> [portainer\_skip\_ssl\_verify](#input\_portainer\_skip\_ssl\_verify) | Set to true to skip TLS certificate verification (useful for self-signed certs) | `bool` | `true` | no |
| <a name="input_portainer_url"></a> [portainer\_url](#input\_portainer\_url) | Default Portainer URL | `string` | `"https://localhost:9443"` | no |
| <a name="input_resource_id"></a> [resource\_id](#input\_resource\_id) | ID of the resource (e.g., stack ID or registry ID) | `string` | `"3"` | no |
| <a name="input_webhook_type"></a> [webhook\_type](#input\_webhook\_type) | Type of the webhook | `number` | `1` | no |

## Outputs

| Name | Description |
|------|-------------|
| <a name="output_webhook_token"></a> [webhook\_token](#output\_webhook\_token) | n/a |
<!-- END_TF_DOCS -->