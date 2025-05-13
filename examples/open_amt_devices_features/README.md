<!-- BEGIN_TF_DOCS -->


## Providers

| Name | Version |
|------|---------|
| <a name="provider_portainer"></a> [portainer](#provider\_portainer) | n/a |

## Resources

| Name | Type |
|------|------|
| [portainer_open_amt_devices_features.example](https://registry.terraform.io/providers/portainer/portainer/latest/docs/resources/open_amt_devices_features) | resource |

## Inputs

| Name | Description | Type | Default | Required |
|------|-------------|------|---------|:--------:|
| <a name="input_device_id"></a> [device\_id](#input\_device\_id) | ID of the AMT managed device | `number` | `42` | no |
| <a name="input_environment_id"></a> [environment\_id](#input\_environment\_id) | Portainer environment ID (agent endpoint) | `number` | `1` | no |
| <a name="input_ider"></a> [ider](#input\_ider) | Enable IDER (IDE Redirection) | `bool` | `true` | no |
| <a name="input_kvm"></a> [kvm](#input\_kvm) | Enable KVM (Keyboard/Video/Mouse) | `bool` | `true` | no |
| <a name="input_portainer_api_key"></a> [portainer\_api\_key](#input\_portainer\_api\_key) | Default Portainer Admin API Key | `string` | n/a | yes |
| <a name="input_portainer_url"></a> [portainer\_url](#input\_portainer\_url) | Default Portainer URL | `string` | n/a | yes |
| <a name="input_redirection"></a> [redirection](#input\_redirection) | Enable redirection | `bool` | `true` | no |
| <a name="input_sol"></a> [sol](#input\_sol) | Enable SOL (Serial Over LAN) | `bool` | `true` | no |
| <a name="input_user_consent"></a> [user\_consent](#input\_user\_consent) | User consent policy (e.g. none, all, kvmOnly) | `string` | `"kvmOnly"` | no |
<!-- END_TF_DOCS -->