<!-- BEGIN_TF_DOCS -->


## Providers

| Name | Version |
|------|---------|
| <a name="provider_portainer"></a> [portainer](#provider\_portainer) | n/a |

## Resources

| Name | Type |
|------|------|
| [portainer_team.test_team](https://registry.terraform.io/providers/portainer/portainer/latest/docs/resources/team) | resource |
| [portainer_team_membership.test_membership](https://registry.terraform.io/providers/portainer/portainer/latest/docs/resources/team_membership) | resource |
| [portainer_user.test_user](https://registry.terraform.io/providers/portainer/portainer/latest/docs/resources/user) | resource |

## Inputs

| Name | Description | Type | Default | Required |
|------|-------------|------|---------|:--------:|
| <a name="input_portainer_api_key"></a> [portainer\_api\_key](#input\_portainer\_api\_key) | Default Portainer Admin API Key | `string` | n/a | yes |
| <a name="input_portainer_url"></a> [portainer\_url](#input\_portainer\_url) | Default Portainer URL | `string` | n/a | yes |
| <a name="input_team_membership_role"></a> [team\_membership\_role](#input\_team\_membership\_role) | Membership role in the team: 1 = leader, 2 = member | `number` | `2` | no |
| <a name="input_team_name"></a> [team\_name](#input\_team\_name) | Name of the Portainer team | `string` | `"test-team"` | no |
| <a name="input_user_ldap"></a> [user\_ldap](#input\_user\_ldap) | Whether the user is an LDAP user | `bool` | `false` | no |
| <a name="input_user_password"></a> [user\_password](#input\_user\_password) | Password for the Portainer user | `string` | `"StrongPassword123!"` | no |
| <a name="input_user_role"></a> [user\_role](#input\_user\_role) | User role: 1 = admin, 2 = standard | `number` | `2` | no |
| <a name="input_user_username"></a> [user\_username](#input\_user\_username) | Username for the Portainer user | `string` | `"testuser"` | no |
<!-- END_TF_DOCS -->