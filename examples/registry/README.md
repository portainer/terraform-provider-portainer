<!-- BEGIN_TF_DOCS -->


## Providers

| Name | Version |
|------|---------|
| <a name="provider_portainer"></a> [portainer](#provider\_portainer) | n/a |

## Resources

| Name | Type |
|------|------|
| [portainer_environment.example](https://registry.terraform.io/providers/portainer/portainer/latest/docs/resources/environment) | resource |
| [portainer_registry.azure](https://registry.terraform.io/providers/portainer/portainer/latest/docs/resources/registry) | resource |
| [portainer_registry.custom](https://registry.terraform.io/providers/portainer/portainer/latest/docs/resources/registry) | resource |
| [portainer_registry.custom_auth](https://registry.terraform.io/providers/portainer/portainer/latest/docs/resources/registry) | resource |
| [portainer_registry.dockerhub](https://registry.terraform.io/providers/portainer/portainer/latest/docs/resources/registry) | resource |
| [portainer_registry.ecr](https://registry.terraform.io/providers/portainer/portainer/latest/docs/resources/registry) | resource |
| [portainer_registry.ecr_anonymous](https://registry.terraform.io/providers/portainer/portainer/latest/docs/resources/registry) | resource |
| [portainer_registry.example](https://registry.terraform.io/providers/portainer/portainer/latest/docs/resources/registry) | resource |
| [portainer_registry.github](https://registry.terraform.io/providers/portainer/portainer/latest/docs/resources/registry) | resource |
| [portainer_registry.github_custom](https://registry.terraform.io/providers/portainer/portainer/latest/docs/resources/registry) | resource |
| [portainer_registry.gitlab](https://registry.terraform.io/providers/portainer/portainer/latest/docs/resources/registry) | resource |
| [portainer_registry.proget](https://registry.terraform.io/providers/portainer/portainer/latest/docs/resources/registry) | resource |
| [portainer_registry.quay](https://registry.terraform.io/providers/portainer/portainer/latest/docs/resources/registry) | resource |
| [portainer_registry_access.example](https://registry.terraform.io/providers/portainer/portainer/latest/docs/resources/registry_access) | resource |
| [portainer_team.example](https://registry.terraform.io/providers/portainer/portainer/latest/docs/resources/team) | resource |

## Inputs

| Name | Description | Type | Default | Required |
|------|-------------|------|---------|:--------:|
| <a name="input_azure_name"></a> [azure\_name](#input\_azure\_name) | Name of the Azure Container Registry. | `string` | `"Azure"` | no |
| <a name="input_azure_password"></a> [azure\_password](#input\_azure\_password) | Password or access key for Azure Container Registry. | `string` | `"azure-password"` | no |
| <a name="input_azure_url"></a> [azure\_url](#input\_azure\_url) | Azure Container Registry URL. | `string` | `"myproject.azurecr.io"` | no |
| <a name="input_azure_username"></a> [azure\_username](#input\_azure\_username) | Username for Azure Container Registry. | `string` | `"azure-user"` | no |
| <a name="input_custom_auth_authentication"></a> [custom\_auth\_authentication](#input\_custom\_auth\_authentication) | Whether authentication is required for the authenticated custom registry. | `bool` | `true` | no |
| <a name="input_custom_auth_name"></a> [custom\_auth\_name](#input\_custom\_auth\_name) | Name of the authenticated custom registry. | `string` | `"Custom Registry Auth"` | no |
| <a name="input_custom_auth_password"></a> [custom\_auth\_password](#input\_custom\_auth\_password) | Password or token for the authenticated custom registry. | `string` | `"custom-registry-password"` | no |
| <a name="input_custom_auth_url"></a> [custom\_auth\_url](#input\_custom\_auth\_url) | URL of the authenticated custom registry. | `string` | `"your-registry.example.com"` | no |
| <a name="input_custom_auth_username"></a> [custom\_auth\_username](#input\_custom\_auth\_username) | Username for the authenticated custom registry. | `string` | `"custom-registry-user"` | no |
| <a name="input_custom_authentication"></a> [custom\_authentication](#input\_custom\_authentication) | Whether authentication is required for the custom registry. | `bool` | `false` | no |
| <a name="input_custom_name"></a> [custom\_name](#input\_custom\_name) | Name of the anonymous custom registry. | `string` | `"Custom Registry"` | no |
| <a name="input_custom_url"></a> [custom\_url](#input\_custom\_url) | URL of the anonymous custom registry. | `string` | `"your-registry.example.com"` | no |
| <a name="input_dockerhub_name"></a> [dockerhub\_name](#input\_dockerhub\_name) | Name of the Docker Hub registry. | `string` | `"DockerHub"` | no |
| <a name="input_dockerhub_password"></a> [dockerhub\_password](#input\_dockerhub\_password) | Docker Hub access token or password. | `string` | `"docker-access-token"` | no |
| <a name="input_dockerhub_url"></a> [dockerhub\_url](#input\_dockerhub\_url) | URL of the Docker Hub registry. | `string` | `"docker.io"` | no |
| <a name="input_dockerhub_username"></a> [dockerhub\_username](#input\_dockerhub\_username) | Docker Hub username. | `string` | `"docker-user"` | no |
| <a name="input_ecr_anonymous_name"></a> [ecr\_anonymous\_name](#input\_ecr\_anonymous\_name) | Name of the anonymous AWS ECR registry. | `string` | `"AWS ECR Anonymous"` | no |
| <a name="input_ecr_anonymous_url"></a> [ecr\_anonymous\_url](#input\_ecr\_anonymous\_url) | URL of the anonymous AWS ECR registry. | `string` | `"123456789.dkr.ecr.us-east-1.amazonaws.com"` | no |
| <a name="input_ecr_name"></a> [ecr\_name](#input\_ecr\_name) | Name of the authenticated AWS ECR registry. | `string` | `"AWS ECR"` | no |
| <a name="input_ecr_password"></a> [ecr\_password](#input\_ecr\_password) | AWS secret key for ECR authentication. | `string` | `"aws-secret-key"` | no |
| <a name="input_ecr_region"></a> [ecr\_region](#input\_ecr\_region) | AWS region where the ECR registry is hosted. | `string` | `"us-east-1"` | no |
| <a name="input_ecr_url"></a> [ecr\_url](#input\_ecr\_url) | URL of the authenticated AWS ECR registry. | `string` | `"123456789.dkr.ecr.us-east-1.amazonaws.com"` | no |
| <a name="input_ecr_username"></a> [ecr\_username](#input\_ecr\_username) | AWS access key for ECR authentication. | `string` | `"aws-access-key"` | no |
| <a name="input_environment_address"></a> [environment\_address](#input\_environment\_address) | Portainer environment address | `string` | `"unix:///var/run/docker.sock"` | no |
| <a name="input_environment_name"></a> [environment\_name](#input\_environment\_name) | Portainer environment name | `string` | `"local-test"` | no |
| <a name="input_environment_type"></a> [environment\_type](#input\_environment\_type) | Portainer environment type | `number` | `1` | no |
| <a name="input_github_authentication"></a> [github\_authentication](#input\_github\_authentication) | Enable authentication for GitHub Container Registry. | `bool` | `true` | no |
| <a name="input_github_custom_authentication"></a> [github\_custom\_authentication](#input\_github\_custom\_authentication) | Enable authentication for GitHub custom CE registry. | `bool` | `true` | no |
| <a name="input_github_custom_name"></a> [github\_custom\_name](#input\_github\_custom\_name) | Name of the GitHub registry (custom CE workaround). | `string` | `"GitHub Registry Custom"` | no |
| <a name="input_github_custom_password"></a> [github\_custom\_password](#input\_github\_custom\_password) | GitHub Personal Access Token for the custom CE registry. | `string` | `"your-github-access-token"` | no |
| <a name="input_github_custom_url"></a> [github\_custom\_url](#input\_github\_custom\_url) | URL of the GitHub registry (custom CE workaround). | `string` | `"ghcr.io"` | no |
| <a name="input_github_custom_username"></a> [github\_custom\_username](#input\_github\_custom\_username) | GitHub username for the custom CE registry. | `string` | `"your-github-username"` | no |
| <a name="input_github_name"></a> [github\_name](#input\_github\_name) | Name of the GitHub Container Registry. | `string` | `"GitHub Registry"` | no |
| <a name="input_github_organisation_name"></a> [github\_organisation\_name](#input\_github\_organisation\_name) | Name of the GitHub organisation (required if github\_use\_organisation = true). | `string` | `"myorg"` | no |
| <a name="input_github_password"></a> [github\_password](#input\_github\_password) | GitHub Personal Access Token used for registry authentication. | `string` | `"your-github-access-token"` | no |
| <a name="input_github_url"></a> [github\_url](#input\_github\_url) | URL of the GitHub Container Registry. | `string` | `"ghcr.io"` | no |
| <a name="input_github_use_organisation"></a> [github\_use\_organisation](#input\_github\_use\_organisation) | Whether to use organisation namespace for GitHub registry. | `bool` | `true` | no |
| <a name="input_github_username"></a> [github\_username](#input\_github\_username) | GitHub username used for registry authentication. | `string` | `"your-github-username"` | no |
| <a name="input_gitlab_instance_url"></a> [gitlab\_instance\_url](#input\_gitlab\_instance\_url) | GitLab instance URL. | `string` | `"https://gitlab.com"` | no |
| <a name="input_gitlab_name"></a> [gitlab\_name](#input\_gitlab\_name) | Name of the GitLab registry. | `string` | `"GitLab Registry"` | no |
| <a name="input_gitlab_password"></a> [gitlab\_password](#input\_gitlab\_password) | Access token or password for the GitLab registry. | `string` | `"gitlab-access-token"` | no |
| <a name="input_gitlab_url"></a> [gitlab\_url](#input\_gitlab\_url) | URL of the GitLab registry. | `string` | `"registry.gitlab.com"` | no |
| <a name="input_gitlab_username"></a> [gitlab\_username](#input\_gitlab\_username) | Username for the GitLab registry. | `string` | `"gitlab-user"` | no |
| <a name="input_portainer_api_key"></a> [portainer\_api\_key](#input\_portainer\_api\_key) | Default Portainer Admin API Key | `string` | n/a | yes |
| <a name="input_portainer_skip_ssl_verify"></a> [portainer\_skip\_ssl\_verify](#input\_portainer\_skip\_ssl\_verify) | Set to true to skip TLS certificate verification (useful for self-signed certs) | `bool` | `true` | no |
| <a name="input_portainer_url"></a> [portainer\_url](#input\_portainer\_url) | Default Portainer URL | `string` | n/a | yes |
| <a name="input_proget_base_url"></a> [proget\_base\_url](#input\_proget\_base\_url) | Base URL of the ProGet registry. | `string` | `"proget.example.com"` | no |
| <a name="input_proget_name"></a> [proget\_name](#input\_proget\_name) | Name of the ProGet registry. | `string` | `"ProGet"` | no |
| <a name="input_proget_password"></a> [proget\_password](#input\_proget\_password) | Password or API token for ProGet registry. | `string` | `"proget-password"` | no |
| <a name="input_proget_url"></a> [proget\_url](#input\_proget\_url) | Full registry URL of the ProGet registry. | `string` | `"proget.example.com/example-registry"` | no |
| <a name="input_proget_username"></a> [proget\_username](#input\_proget\_username) | Username for ProGet authentication. | `string` | `"proget-user"` | no |
| <a name="input_public_ip"></a> [public\_ip](#input\_public\_ip) | Public IP/URL for Portainer PublicURL | `string` | `"unix:///var/run/docker.sock"` | no |
| <a name="input_quay_authentication"></a> [quay\_authentication](#input\_quay\_authentication) | Enable authentication for Quay.io registry. | `bool` | `true` | no |
| <a name="input_quay_name"></a> [quay\_name](#input\_quay\_name) | Name of the Quay.io registry. | `string` | `"Quay"` | no |
| <a name="input_quay_organisation_name"></a> [quay\_organisation\_name](#input\_quay\_organisation\_name) | Name of the Quay.io organisation (required if quay\_use\_organisation = true). | `string` | `"myorg"` | no |
| <a name="input_quay_password"></a> [quay\_password](#input\_quay\_password) | Access token or password for Quay.io registry. | `string` | `"quay-token"` | no |
| <a name="input_quay_url"></a> [quay\_url](#input\_quay\_url) | URL of the Quay.io registry. | `string` | `"quay.io"` | no |
| <a name="input_quay_use_organisation"></a> [quay\_use\_organisation](#input\_quay\_use\_organisation) | Whether to use organisation namespace for Quay.io registry. | `bool` | `true` | no |
| <a name="input_quay_username"></a> [quay\_username](#input\_quay\_username) | Username for Quay.io authentication. | `string` | `"quay-user"` | no |
| <a name="input_team_name"></a> [team\_name](#input\_team\_name) | Name of the test team. | `string` | `"Test Team for Registry Access"` | no |
<!-- END_TF_DOCS -->