# Portainer Provider

A [Terraform](https://www.terraform.io) provider to manage [Portainer](https://www.portainer.io/) resources via its REST API using Terraform.

It supports provisioning and configuration of Portainer users and will be extended to support other objects such as teams, compose/stacks, endpoints, and access control.

## 🏷️ Provider Support
| Provider       | Provider Support Status              |
|----------------|--------------------------------------|
| [Terraform](https://registry.terraform.io/providers/portainer/portainer/latest)      | ![Done](https://img.shields.io/badge/status-done-brightgreen)           |
| [OpenTofu](https://search.opentofu.org/provider/portainer/portainer/latest)       | ![Done](https://img.shields.io/badge/status-done-brightgreen) |

## ⚙️ Example Provider Configuration
```hcl
provider "portainer" {
  endpoint = "..."
  api_key  = "..."
  skip_ssl_verify  = true # optional (default value is `false`)
}
```

## 🔐 Authentication
- Static API key

Static credentials can be provided by adding the `api_key` variables in-line in the Portainer provider block:

> **Authentication:** This provider supports only **API keys** via the `X-API-Key` header. JWT tokens curentlly are not supported in this provider.

Usage:

```hcl
provider "portainer" {
  api_key  = "..."
}
```
### Environment variables
You can provide your configuration via the environment variables representing your minio credentials:

```hcl
$ export PORTAINER_ENDPOINT="http://portainer.example.com"
$ export PORTAINER_API_KEY="your-api-key"
$ export PORTAINER_SKIP_SSL_VERIFY=true
```

### Arguments Reference
| Name       | Type   | Required | Description                                                                 |
|------------|--------|----------|-----------------------------------------------------------------------------|
| `endpoint` | string | ✅ yes   | The URL of the Portainer instance. `/api` will be appended automatically if missing. |
| `api_key`  | string | ✅ yes   | API key used to authenticate requests.                                      |
| `skip_ssl_verify` | boolean | ❌ no | 	Set to `true` to skip TLS certificate verification (useful for self-signed certs). Default: `false` |


## 🧩 Supported Resources
| Resource                   | Status                                                                 |
|----------------------------|------------------------------------------------------------------------|
| `portainer_user`           | ![Done](https://img.shields.io/badge/status-done-brightgreen)         |
| `portainer_team`           | ![Done](https://img.shields.io/badge/status-done-brightgreen)         |
| `portainer_team_membrship` | ![Done](https://img.shields.io/badge/status-done-brightgreen)         |
| `portainer_environment`    | ![Done](https://img.shields.io/badge/status-done-brightgreen)         |
| `portainer_tag`            | ![Done](https://img.shields.io/badge/status-done-brightgreen)         |
| `portainer_endpoint_group` | ![Done](https://img.shields.io/badge/status-done-brightgreen)         |
| `portainer_registry`       | ![Done](https://img.shields.io/badge/status-done-brightgreen)         |
| `portainer_backup`         | ![Done](https://img.shields.io/badge/status-done-brightgreen)         |
| `portainer_backup_s3`      | ![Done](https://img.shields.io/badge/status-done-brightgreen)         |
| `portainer_auth`           | ![Done](https://img.shields.io/badge/status-done-brightgreen)         |
| `portainer_edge_group`     | ![Done](https://img.shields.io/badge/status-done-brightgreen)         |
| `portainer_edge_stack`     | ![Done](https://img.shields.io/badge/status-done-brightgreen)         |
| `portainer_edge_job`       | ![Done](https://img.shields.io/badge/status-done-brightgreen)         |
| `portainer_stack`          | ![Done](https://img.shields.io/badge/status-done-brightgreen)         |
| `portainer_custom_template`| ![Done](https://img.shields.io/badge/status-done-brightgreen)         |
| `portainer_container_exec` | ![Done](https://img.shields.io/badge/status-done-brightgreen)         |
| `portainer_docker_network` | ![Done](https://img.shields.io/badge/status-done-brightgreen)         |
| `portainer_docker_image`   | ![Done](https://img.shields.io/badge/status-done-brightgreen)         |
| `portainer_docker_volume`  | ![Done](https://img.shields.io/badge/status-done-brightgreen)         |
| `portainer_docker_secret`  | ![Done](https://img.shields.io/badge/status-done-brightgreen)         |
| `portainer_docker_config`  | ![Done](https://img.shields.io/badge/status-done-brightgreen)         |
| `portainer_open_amt`       | ![Done](https://img.shields.io/badge/status-done-brightgreen)         |
| `portainer_settings`       | ![Done](https://img.shields.io/badge/status-done-brightgreen)         |
| `portainer_endpoint_settings`| ![Done](https://img.shields.io/badge/status-done-brightgreen)       |
| `portainer_endpoint_service_update`| ![Done](https://img.shields.io/badge/status-done-brightgreen)       |
| `portainer_endpoint_snapshot`| ![Done](https://img.shields.io/badge/status-done-brightgreen)      |
| `portainer_endpoint_association`| ![Done](https://img.shields.io/badge/status-done-brightgreen)      |
| `portainer_webhook`        | ![Done](https://img.shields.io/badge/status-done-brightgreen)         |
| `portainer_webhook_execute`| ![Done](https://img.shields.io/badge/status-done-brightgreen)         |
| `portainer_resource_control`| ![Done](https://img.shields.io/badge/status-done-brightgreen) |
| `portainer_licenses`       | ![Done](https://img.shields.io/badge/status-done-brightgreen)         |
| `portainer_cloud_credentials`| ![Done](https://img.shields.io/badge/status-done-brightgreen)       |
| `portainer_kubernetes_delete_object`                  | ![Done](https://img.shields.io/badge/status-done-brightgreen) |
| `portainer_kubernetes_helm`                           | ![Done](https://img.shields.io/badge/status-done-brightgreen) |
| `portainer_kubernetes_ingresscontrollers`             | ![Done](https://img.shields.io/badge/status-done-brightgreen) |
| `portainer_kubernetes_namespace_ingresscontrollers`   | ![Done](https://img.shields.io/badge/status-done-brightgreen) |
| `portainer_kubernetes_namespace_system`               | ![Done](https://img.shields.io/badge/status-done-brightgreen) |
| `portainer_kubernetes_namespace`                      | ![Done](https://img.shields.io/badge/status-done-brightgreen) |
| `portainer_kubernetes_cronjob`                        | ![Done](https://img.shields.io/badge/status-done-brightgreen) |
| `portainer_kubernetes_job`                            | ![Done](https://img.shields.io/badge/status-done-brightgreen) |
| `portainer_kubernetes_service_account`                | ![Done](https://img.shields.io/badge/status-done-brightgreen) |
| `portainer_kubernetes_configmaps`                     | ![Done](https://img.shields.io/badge/status-done-brightgreen) |
| `portainer_kubernetes_secret`                         | ![Done](https://img.shields.io/badge/status-done-brightgreen) |
| `portainer_kubernetes_service`                        | ![Done](https://img.shields.io/badge/status-done-brightgreen) |
| `portainer_kubernetes_role`                           | ![Done](https://img.shields.io/badge/status-done-brightgreen) |
| `portainer_kubernetes_rolebinding`                    | ![Done](https://img.shields.io/badge/status-done-brightgreen) |
| `portainer_kubernetes_clusterrole`                    | ![Done](https://img.shields.io/badge/status-done-brightgreen) |
| `portainer_kubernetes_clusterrolebinding`             | ![Done](https://img.shields.io/badge/status-done-brightgreen) |
| `portainer_kubernetes_application`                    | ![Done](https://img.shields.io/badge/status-done-brightgreen) |
| `portainer_kubernetes_ingress`                        | ![Done](https://img.shields.io/badge/status-done-brightgreen) |
| `portainer_kubernetes_volume`                         | ![Done](https://img.shields.io/badge/status-done-brightgreen) |
| `portainer_kubernetes_storage`                        | ![Done](https://img.shields.io/badge/status-done-brightgreen) |

---

### 💡 Missing a resource?
Is there a Portainer resource you'd like to see supported?

👉 [Open an issue](https://github.com/portainer/terraform-provider-portainer/issues/new?template=feature_request.md) and we’ll consider it for implementation — or even better, submit a [Pull Request](https://github.com/portainer/terraform-provider-portainer/pulls) to contribute directly!

📘 See [CONTRIBUTING.md](https://github.com/portainer/terraform-provider-portainer/blob/main/.github/CONTRIBUTING.md) for guidelines.

## ✅ Daily End-to-End Testing
To ensure maximum reliability and functionality of this provider, **automated end-to-end tests are executed every day** via GitHub Actions.

These tests run against a real Portainer instance (started using docker compose) and validate the majority of supported resources using real Terraform plans and applies.

> 💡 This helps catch regressions early and ensures the provider remains fully operational and compatible with the Portainer API.

## ♻️ Terraform Import Guide
You can import existing Portainer-managed resources into Terraform using the `terraform import` command. This is useful for adopting GitOps practices or migrating manually created resources into code.

### ✅ General Syntax
```hcl
terraform import <RESOURCE_TYPE>.<NAME> <ID>
```
- `<RESOURCE_TYPE>` – the Terraform resource type, e.g., portainer_tag
- `<NAME>` – the local name you've chosen in your .tf file
- `<ID>` – the Portainer object ID (usually numeric)

### 🛠 Example: Import an existing tag
Let's say you already have a tag with ID 3 in Portainer. First, define it in your configuration:
```hcl
resource "portainer_tag" "existing_tag" {
  name = "production"
}
```
Then run the import:
```hcl
terraform import portainer_tag.existing_tag 3
```
Terraform will fetch the current state of the resource and start managing it. You can now safely plan and apply updates from Terraform.

### 📦 Auto-generate Terraform configuration
After a successful import, you can automatically generate the resource definition from the Terraform state:
```hcl
./generate-tf.sh
```
This script reads the current Terraform state and generates a file named `generated.tf` with the proper configuration of the imported resources. You can copy or refactor the output into your main Terraform files.
> ℹ️ Note: Only resources with import support listed as ✅ in the table above can be imported.

## 📜 License
This module is 100% Open Source and all versions of this provider starting from v2.0.0 are distributed under the AGPL-3.0 License. See [LICENSE](https://github.com/portainer/terraform-provider-portainer/blob/main/LICENSE) for more information.

## 🌐 Acknowledgements
- [HashiCorp Terraform](https://www.hashicorp.com/products/terraform)
- [Portainer](https://portainer.io)
- [OpenTofu](https://opentofu.org/)
- [Docker](https://www.docker.com/)
