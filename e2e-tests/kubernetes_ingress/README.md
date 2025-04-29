<!-- BEGIN_TF_DOCS -->


## Providers

| Name | Version |
|------|---------|
| <a name="provider_portainer"></a> [portainer](#provider\_portainer) | n/a |

## Resources

| Name | Type |
|------|------|
| [portainer_kubernetes_ingresses.example](https://registry.terraform.io/providers/portainer/portainer/latest/docs/resources/kubernetes_ingresses) | resource |

## Inputs

| Name | Description | Type | Default | Required |
|------|-------------|------|---------|:--------:|
| <a name="input_annotations"></a> [annotations](#input\_annotations) | Annotations to be applied to the ingress | `map(string)` | <pre>{<br/>  "kubernetes.io/ingress.class": "nginx"<br/>}</pre> | no |
| <a name="input_class_name"></a> [class\_name](#input\_class\_name) | Ingress controller class name (e.g., nginx) | `string` | `"nginx"` | no |
| <a name="input_environment_id"></a> [environment\_id](#input\_environment\_id) | Portainer environment (endpoint) ID | `number` | `4` | no |
| <a name="input_hosts"></a> [hosts](#input\_hosts) | List of hostnames for the ingress | `list(string)` | <pre>[<br/>  "example.com"<br/>]</pre> | no |
| <a name="input_ingress_name"></a> [ingress\_name](#input\_ingress\_name) | Name of the ingress resource | `string` | `"example-ingress"` | no |
| <a name="input_labels"></a> [labels](#input\_labels) | Labels to be applied to the ingress | `map(string)` | <pre>{<br/>  "app": "nginx"<br/>}</pre> | no |
| <a name="input_namespace"></a> [namespace](#input\_namespace) | Kubernetes namespace where the ingress will be created | `string` | `"default"` | no |
| <a name="input_path"></a> [path](#input\_path) | Ingress path | `string` | `"/"` | no |
| <a name="input_path_host"></a> [path\_host](#input\_path\_host) | Host for ingress path | `string` | `"example.com"` | no |
| <a name="input_path_type"></a> [path\_type](#input\_path\_type) | Type of the path (e.g., Prefix) | `string` | `"Prefix"` | no |
| <a name="input_portainer_api_key"></a> [portainer\_api\_key](#input\_portainer\_api\_key) | Default Portainer Admin API Key | `string` | `"ptr_xrP7XWqfZEOoaCJRu5c8qKaWuDtVc2Zb07Q5g22YpS8="` | no |
| <a name="input_portainer_skip_ssl_verify"></a> [portainer\_skip\_ssl\_verify](#input\_portainer\_skip\_ssl\_verify) | Set to true to skip TLS certificate verification (useful for self-signed certs) | `bool` | `true` | no |
| <a name="input_portainer_url"></a> [portainer\_url](#input\_portainer\_url) | Default Portainer URL | `string` | `"https://localhost:9443"` | no |
| <a name="input_service_name"></a> [service\_name](#input\_service\_name) | Name of the Kubernetes service | `string` | `"nginx-service"` | no |
| <a name="input_service_port"></a> [service\_port](#input\_service\_port) | Port number for the service | `number` | `80` | no |
| <a name="input_tls_hosts"></a> [tls\_hosts](#input\_tls\_hosts) | List of TLS hosts | `list(string)` | <pre>[<br/>  "example.com"<br/>]</pre> | no |
| <a name="input_tls_secret_name"></a> [tls\_secret\_name](#input\_tls\_secret\_name) | Secret name for TLS | `string` | `"example-tls"` | no |
<!-- END_TF_DOCS -->