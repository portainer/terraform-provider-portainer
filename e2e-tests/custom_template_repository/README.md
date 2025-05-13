<!-- BEGIN_TF_DOCS -->


## Providers

| Name | Version |
|------|---------|
| <a name="provider_portainer"></a> [portainer](#provider\_portainer) | n/a |

## Resources

| Name | Type |
|------|------|
| [portainer_custom_template.example_string](https://registry.terraform.io/providers/portainer/portainer/latest/docs/resources/custom_template) | resource |

## Inputs

| Name | Description | Type | Default | Required |
|------|-------------|------|---------|:--------:|
| <a name="input_custom_template_compose_file_path"></a> [custom\_template\_compose\_file\_path](#input\_custom\_template\_compose\_file\_path) | Inline file content for the template (YAML/Compose) | `string` | `"docker-compose.agent.yml"` | no |
| <a name="input_custom_template_description"></a> [custom\_template\_description](#input\_custom\_template\_description) | Description of the custom template | `string` | `"Deploy Portainer Agent container"` | no |
| <a name="input_custom_template_note"></a> [custom\_template\_note](#input\_custom\_template\_note) | Note that appears in the UI | `string` | `"Runs Portainer Agent container with required mounts"` | no |
| <a name="input_custom_template_platform"></a> [custom\_template\_platform](#input\_custom\_template\_platform) | Platform: 1 = linux, 2 = windows | `number` | `1` | no |
| <a name="input_custom_template_repository_reference"></a> [custom\_template\_repository\_reference](#input\_custom\_template\_repository\_reference) | Whether this is an Edge template | `string` | `"refs/heads/main"` | no |
| <a name="input_custom_template_repository_url"></a> [custom\_template\_repository\_url](#input\_custom\_template\_repository\_url) | Is Compose format (true/false) | `string` | `"https://github.com/portainer/terraform-provider-portainer"` | no |
| <a name="input_custom_template_title"></a> [custom\_template\_title](#input\_custom\_template\_title) | Title of the custom template | `string` | `"Portainer Agent"` | no |
| <a name="input_custom_template_type"></a> [custom\_template\_type](#input\_custom\_template\_type) | Stack type: 1 = swarm, 2 = compose, 3 = kubernetes | `number` | `2` | no |
| <a name="input_portainer_api_key"></a> [portainer\_api\_key](#input\_portainer\_api\_key) | Default Portainer Admin API Key | `string` | `"ptr_xrP7XWqfZEOoaCJRu5c8qKaWuDtVc2Zb07Q5g22YpS8="` | no |
| <a name="input_portainer_skip_ssl_verify"></a> [portainer\_skip\_ssl\_verify](#input\_portainer\_skip\_ssl\_verify) | Set to true to skip TLS certificate verification (useful for self-signed certs) | `bool` | `true` | no |
| <a name="input_portainer_url"></a> [portainer\_url](#input\_portainer\_url) | Default Portainer URL | `string` | `"https://localhost:9443"` | no |
<!-- END_TF_DOCS -->