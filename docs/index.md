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

  # Option 1: API key authentication
  api_key  = "..."

  # Option 2: Username/password authentication (generates JWT token internally)
  # api_user     = "..."
  # api_password = "..."

  skip_ssl_verify  = true # optional (default value is `false`)
}
```

## 🔐 Authentication
The Portainer Terraform provider supports two authentication methods:
1. **API Key** (via `X-API-Key` header)
2. **Username & Password** (via `/api/auth` → JWT token internally used)

Only one method is required – if both are provided, `api_key` takes precedence.

#### Usage – API Key:

```hcl
provider "portainer" {
  api_key  = "..."
}
```

#### Usage – Username & Password:

```hcl
provider "portainer" {
  api_user     = "..."
  api_password = "..."
}
```

### Environment variables
You can also configure the provider via environment variables:

#### API key method
```hcl
$ export PORTAINER_ENDPOINT="https://portainer.example.com"
$ export PORTAINER_API_KEY="your-api-key"
$ export PORTAINER_SKIP_SSL_VERIFY=true
```
#### Username and password method
```hcl
$ export PORTAINER_ENDPOINT="https://portainer.example.com"
$ export PORTAINER_USER="admin"
$ export PORTAINER_PASSWORD="your-password"
$ export PORTAINER_SKIP_SSL_VERIFY=true
```

## Arguments Reference
| Name              | Type    | Required | Description                                                                                        |
| ----------------- | ------- | -------- | ---------------------------------------------------------------------------------------------------|
| `endpoint`        | string  | ✅ yes   | URL of the Portainer instance. `/api` will be appended automatically if missing.                   |
| `api_key`         | string  | ❌ no    | API key for authentication. Mutually exclusive with `api_user` and `api_password`.                 |
| `api_user`        | string  | ❌ no    | Username for authentication (must be used with `api_password`). Mutually exclusive with `api_key`. |
| `api_password`    | string  | ❌ no    | Password for authentication (must be used with `api_user`). Mutually exclusive with `api_key`.     |
| `skip_ssl_verify` | boolean | ❌ no    | Skip TLS certificate verification (useful for self-signed certs). Default: `false`.                |

## 🧩 Supported Resources
| Resource                                       | Status                                                                 |
|------------------------------------------------|------------------------------------------------------------------------|
| `portainer_auth`                               | ![Done](https://img.shields.io/badge/status-done-brightgreen)         |
| `portainer_backup`                             | ![Done](https://img.shields.io/badge/status-done-brightgreen)         |
| `portainer_backup_s3`                          | ![Done](https://img.shields.io/badge/status-done-brightgreen)         |
| `portainer_chat`                               | ![Done](https://img.shields.io/badge/status-done-brightgreen)         |
| `portainer_cloud_credentials`                  | ![Done](https://img.shields.io/badge/status-done-brightgreen)         |
| `portainer_cloud_provider_provision`           | ![Done](https://img.shields.io/badge/status-done-brightgreen)         |
| `portainer_compose_convert`                    | ![Done](https://img.shields.io/badge/status-done-brightgreen)         |
| `portainer_container_exec`                     | ![Done](https://img.shields.io/badge/status-done-brightgreen)         |
| `portainer_custom_template`                    | ![Done](https://img.shields.io/badge/status-done-brightgreen)         |
| `portainer_docker_config`                      | ![Done](https://img.shields.io/badge/status-done-brightgreen)         |
| `portainer_docker_image`                       | ![Done](https://img.shields.io/badge/status-done-brightgreen)         |
| `portainer_docker_network`                     | ![Done](https://img.shields.io/badge/status-done-brightgreen)         |
| `portainer_docker_plugin`                      | ![Done](https://img.shields.io/badge/status-done-brightgreen)         |
| `portainer_docker_node`                        | ![Done](https://img.shields.io/badge/status-done-brightgreen)         |
| `portainer_docker_secret`                      | ![Done](https://img.shields.io/badge/status-done-brightgreen)         |
| `portainer_docker_volume`                      | ![Done](https://img.shields.io/badge/status-done-brightgreen)         |
| `portainer_edge_configurations`                | ![Done](https://img.shields.io/badge/status-done-brightgreen)         |
| `portainer_edge_group`                         | ![Done](https://img.shields.io/badge/status-done-brightgreen)         |
| `portainer_edge_job`                           | ![Done](https://img.shields.io/badge/status-done-brightgreen)         |
| `portainer_edge_stack`                         | ![Done](https://img.shields.io/badge/status-done-brightgreen)         |
| `portainer_edge_stack_webhook`                 | ![Done](https://img.shields.io/badge/status-done-brightgreen)         |
| `portainer_edge_update_schedules`              | ![Done](https://img.shields.io/badge/status-done-brightgreen)         |
| `portainer_endpoint_association`               | ![Done](https://img.shields.io/badge/status-done-brightgreen)         |
| `portainer_endpoint_group`                     | ![Done](https://img.shields.io/badge/status-done-brightgreen)         |
| `portainer_endpoint_service_update`            | ![Done](https://img.shields.io/badge/status-done-brightgreen)         |
| `portainer_endpoint_settings`                  | ![Done](https://img.shields.io/badge/status-done-brightgreen)         |
| `portainer_endpoint_snapshot`                  | ![Done](https://img.shields.io/badge/status-done-brightgreen)         |
| `portainer_endpoints_edge_generate_key`        | ![Done](https://img.shields.io/badge/status-done-brightgreen)         |
| `portainer_environment`                        | ![Done](https://img.shields.io/badge/status-done-brightgreen)         |
| `portainer_kubernetes_application`             | ![Done](https://img.shields.io/badge/status-done-brightgreen)         |
| `portainer_kubernetes_clusterrole`             | ![Done](https://img.shields.io/badge/status-done-brightgreen)         |
| `portainer_kubernetes_clusterrolebinding`      | ![Done](https://img.shields.io/badge/status-done-brightgreen)         |
| `portainer_kubernetes_configmaps`              | ![Done](https://img.shields.io/badge/status-done-brightgreen)         |
| `portainer_kubernetes_cronjob`                 | ![Done](https://img.shields.io/badge/status-done-brightgreen)         |
| `portainer_kubernetes_delete_object`           | ![Done](https://img.shields.io/badge/status-done-brightgreen)         |
| `portainer_kubernetes_helm`                    | ![Done](https://img.shields.io/badge/status-done-brightgreen)         |
| `portainer_kubernetes_ingress`                 | ![Done](https://img.shields.io/badge/status-done-brightgreen)         |
| `portainer_kubernetes_ingresscontrollers`      | ![Done](https://img.shields.io/badge/status-done-brightgreen)         |
| `portainer_kubernetes_job`                     | ![Done](https://img.shields.io/badge/status-done-brightgreen)         |
| `portainer_kubernetes_namespace`               | ![Done](https://img.shields.io/badge/status-done-brightgreen)         |
| `portainer_kubernetes_namespace_ingresscontrollers` | ![Done](https://img.shields.io/badge/status-done-brightgreen)     |
| `portainer_kubernetes_namespace_system`        | ![Done](https://img.shields.io/badge/status-done-brightgreen)         |
| `portainer_kubernetes_role`                    | ![Done](https://img.shields.io/badge/status-done-brightgreen)         |
| `portainer_kubernetes_rolebinding`             | ![Done](https://img.shields.io/badge/status-done-brightgreen)         |
| `portainer_kubernetes_secret`                  | ![Done](https://img.shields.io/badge/status-done-brightgreen)         |
| `portainer_kubernetes_service`                 | ![Done](https://img.shields.io/badge/status-done-brightgreen)         |
| `portainer_kubernetes_service_account`         | ![Done](https://img.shields.io/badge/status-done-brightgreen)         |
| `portainer_kubernetes_storage`                 | ![Done](https://img.shields.io/badge/status-done-brightgreen)         |
| `portainer_kubernetes_volume`                  | ![Done](https://img.shields.io/badge/status-done-brightgreen)         |
| `portainer_licenses`                           | ![Done](https://img.shields.io/badge/status-done-brightgreen)         |
| `portainer_open_amt`                           | ![Done](https://img.shields.io/badge/status-done-brightgreen)         |
| `portainer_open_amt_activate`                  | ![Done](https://img.shields.io/badge/status-done-brightgreen)         |
| `portainer_open_amt_devices_action`            | ![Done](https://img.shields.io/badge/status-done-brightgreen)         |
| `portainer_open_amt_devices_features`          | ![Done](https://img.shields.io/badge/status-done-brightgreen)         |
| `portainer_registry`                           | ![Done](https://img.shields.io/badge/status-done-brightgreen)         |
| `portainer_resource_control`                   | ![Done](https://img.shields.io/badge/status-done-brightgreen)         |
| `portainer_settings`                           | ![Done](https://img.shields.io/badge/status-done-brightgreen)         |
| `portainer_settings_experimental`              | ![Done](https://img.shields.io/badge/status-done-brightgreen)         |
| `portainer_ssl`                                | ![Done](https://img.shields.io/badge/status-done-brightgreen)         |
| `portainer_sshkeygen`                          | ![Done](https://img.shields.io/badge/status-done-brightgreen)         |
| `portainer_stack`                              | ![Done](https://img.shields.io/badge/status-done-brightgreen)         |
| `portainer_stack_associate`                    | ![Done](https://img.shields.io/badge/status-done-brightgreen)         |
| `portainer_stack_webhook`                      | ![Done](https://img.shields.io/badge/status-done-brightgreen)         |
| `portainer_support_debug_log`                  | ![Done](https://img.shields.io/badge/status-done-brightgreen)         |
| `portainer_tag`                                | ![Done](https://img.shields.io/badge/status-done-brightgreen)         |
| `portainer_team`                               | ![Done](https://img.shields.io/badge/status-done-brightgreen)         |
| `portainer_team_membership`                    | ![Done](https://img.shields.io/badge/status-done-brightgreen)         |
| `portainer_tls`                                | ![Done](https://img.shields.io/badge/status-done-brightgreen)         |
| `portainer_user`                               | ![Done](https://img.shields.io/badge/status-done-brightgreen)         |
| `portainer_webhook`                            | ![Done](https://img.shields.io/badge/status-done-brightgreen)         |
| `portainer_webhook_execute`                    | ![Done](https://img.shields.io/badge/status-done-brightgreen)         |


### 🐳 Podman Support via Docker Resources

[Podman is compatible with the Docker API](https://docs.podman.io/en/latest/_static/api.html), which means you can use existing `portainer_docker_*` resources with Podman – **no special `portainer_podman_*` resources are needed**.

✅ **Use Docker resources for Podman**  
Podman works out of the box with most `portainer_docker_*` Terraform resources.

> ⚠️ **Note**:  
> Podman **does not support Docker Swarm** – any swarm-based features are **not compatible**.

### Docker Compose to Kubernetes Conversion

You can now use the `portainer_compose_convert` resource to convert Docker Compose YAML directly into Kubernetes manifests using [Kompose](https://github.com/kubernetes/kompose).

This is especially useful when migrating applications from Docker standalone or Swarm mode to Kubernetes – while keeping your deployment definitions fully managed as code in Terraform.
> ℹ️ The resource uses Kompose internally and supports both the installed CLI binary.
See full documentation: [docs/resources/compose_convert.md](https://github.com/portainer/terraform-provider-portainer/tree/main/docs/resources/compose_convert.md)

---

### 💡 Missing a resource?
Is there a Portainer resource you'd like to see supported?

👉 [Open an issue](https://github.com/portainer/terraform-provider-portainer/issues/new?template=feature_request.md) and we’ll consider it for implementation — or even better, submit a [Pull Request](https://github.com/portainer/terraform-provider-portainer/pulls) to contribute directly!

📘 See [CONTRIBUTING.md](https://github.com/portainer/terraform-provider-portainer/blob/main/.github/CONTRIBUTING.md) for guidelines.

## 💬 Community & Feedback
Have questions, suggestions or want to contribute ideas?  
Join the **Portainer Community Slack** and hop into the [`#portainer-terraform`](https://app.slack.com/client/T2AGA35A4/C08NHK6PLUT) channel!

Want to report issues, submit pull requests or browse the source code?  
Check out the [GitHub Repository](https://github.com/portainer/terraform-provider-portainer) for this provider.

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
This module is 100% Open Source and is distributed under the MIT License.  
See the [LICENSE](https://github.com/portainer/terraform-provider-portainer/blob/main/LICENSE) file for more information.

## 🌐 Acknowledgements
- [HashiCorp Terraform](https://www.hashicorp.com/products/terraform)
- [Portainer](https://portainer.io)
- [OpenTofu](https://opentofu.org/)
- [Docker](https://www.docker.com/)
