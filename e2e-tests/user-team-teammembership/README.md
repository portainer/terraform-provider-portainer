<!-- BEGIN_TF_DOCS -->


## Providers

| Name | Version |
|------|---------|
| <a name="provider_portainer"></a> [portainer](#provider\_portainer) | 0.1.0 |

## Resources

| Name | Type |
|------|------|
| [portainer_team.your_team](https://registry.terraform.io/providers/portainer/portainer/latest/docs/resources/team) | resource |
| [portainer_team_membership.your_membership](https://registry.terraform.io/providers/portainer/portainer/latest/docs/resources/team_membership) | resource |
| [portainer_user.your_user](https://registry.terraform.io/providers/portainer/portainer/latest/docs/resources/user) | resource |

## Inputs

| Name | Description | Type | Default | Required |
|------|-------------|------|---------|:--------:|
| <a name="input_portainer_api_key"></a> [portainer\_api\_key](#input\_portainer\_api\_key) | Default Portainer Admin API Key | `string` | `"ptr_xrP7XWqfZEOoaCJRu5c8qKaWuDtVc2Zb07Q5g22YpS8="` | no |
| <a name="input_portainer_skip_ssl_verify"></a> [portainer\_skip\_ssl\_verify](#input\_portainer\_skip\_ssl\_verify) | Set to true to skip TLS certificate verification (useful for self-signed certs) | `bool` | `true` | no |
| <a name="input_portainer_team_name"></a> [portainer\_team\_name](#input\_portainer\_team\_name) | Portainer Team Name | `string` | `"your-team"` | no |
| <a name="input_portainer_url"></a> [portainer\_url](#input\_portainer\_url) | Default Portainer URL | `string` | `"https://localhost:9443"` | no |
| <a name="input_portainer_user_password"></a> [portainer\_user\_password](#input\_portainer\_user\_password) | Portainer password used for resource provisioning | `string` | `"your-user-password"` | no |
| <a name="input_portainer_user_role"></a> [portainer\_user\_role](#input\_portainer\_user\_role) | Role to assign to the Portainer user | `number` | `2` | no |
| <a name="input_portainer_user_username"></a> [portainer\_user\_username](#input\_portainer\_user\_username) | Portainer username used for resource provisioning | `string` | `"your-user"` | no |
| <a name="input_team_membership_role"></a> [team\_membership\_role](#input\_team\_membership\_role) | Membership role in the team: 1 = leader, 2 = member | `number` | `2` | no |
<!-- END_TF_DOCS -->