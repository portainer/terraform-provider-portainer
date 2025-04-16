<!-- BEGIN_TF_DOCS -->


## Providers

| Name | Version |
|------|---------|
| <a name="provider_portainer"></a> [portainer](#provider\_portainer) | n/a |

## Resources

| Name | Type |
|------|------|
| [portainer_registry.registry](https://registry.terraform.io/providers/portainer/portainer/latest/docs/resources/registry) | resource |

## Inputs

| Name | Description | Type | Default | Required |
|------|-------------|------|---------|:--------:|
| <a name="input_portainer_api_key"></a> [portainer\_api\_key](#input\_portainer\_api\_key) | Default Portainer Admin API Key | `string` | n/a | yes |
| <a name="input_portainer_registry_name"></a> [portainer\_registry\_name](#input\_portainer\_registry\_name) | Název registru | `string` | n/a | yes |
| <a name="input_portainer_registry_password"></a> [portainer\_registry\_password](#input\_portainer\_registry\_password) | Heslo nebo token pro přístup do registru | `string` | n/a | yes |
| <a name="input_portainer_registry_type"></a> [portainer\_registry\_type](#input\_portainer\_registry\_type) | Typ registru (např. 6 = Docker Hub) | `number` | n/a | yes |
| <a name="input_portainer_registry_url"></a> [portainer\_registry\_url](#input\_portainer\_registry\_url) | URL adresa registru | `string` | n/a | yes |
| <a name="input_portainer_registry_username"></a> [portainer\_registry\_username](#input\_portainer\_registry\_username) | Uživatelské jméno pro registr | `string` | n/a | yes |
| <a name="input_portainer_url"></a> [portainer\_url](#input\_portainer\_url) | Default Portainer URL | `string` | n/a | yes |
<!-- END_TF_DOCS -->