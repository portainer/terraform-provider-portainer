
<p align="center">
  <a href="https://registry.terraform.io/providers/portainer/portainer/latest/docs">
    <img src="https://www.terraform.io/_next/static/media/terraform-community_on-light.cda79e7c.svg" alt="Terraform Logo" width="200">
  </a>
  &nbsp;&nbsp;&nbsp;
  <a href="https://github.com/portainer/terraform-provider-portainer">
    <img src="https://www.portainer.io/hubfs/portainer-logo-black.svg" alt="portainer-provider-terraform" width="200">
  </a>
  &nbsp;&nbsp;&nbsp;
  <a href="https://search.opentofu.org/provider/portainer/portainer/latest">
    <img src="https://raw.githubusercontent.com/opentofu/brand-artifacts/main/full/transparent/SVG/on-dark.svg#gh-dark-mode-only" alt="portainer-provider-opentofu" width="200">
  </a>
  <h3 align="center" style="font-weight: bold">Terraform Provider for Portainer</h3>
  <p align="center">
    <a href="https://github.com/portainer/terraform-provider-portainer/graphs/contributors">
      <img alt="Contributors" src="https://img.shields.io/github/contributors/portainer/terraform-provider-portainer">
    </a>
    <a href="https://golang.org/doc/devel/release.html">
      <img alt="GitHub go.mod Go version" src="https://img.shields.io/github/go-mod/go-version/portainer/terraform-provider-portainer">
    </a>
    <a href="https://github.com/portainer/terraform-provider-portainer/actions?query=workflow%3Arelease">
      <img alt="GitHub Workflow Status" src="https://img.shields.io/github/actions/workflow/status/portainer/terraform-provider-portainer/release.yml?tag=latest&label=release">
    </a>
    <a href="https://github.com/portainer/terraform-provider-portainer/releases">
      <img alt="GitHub release (latest by date including pre-releases)" src="https://img.shields.io/github/v/release/portainer/terraform-provider-portainer?include_prereleases">
    </a>
  </p>
  <p align="center">
    <a href="https://github.com/portainer/terraform-provider-portainer/tree/main/docs"><strong>Explore the docs »</strong></a>
  </p>
</p>

# Portainer Terraform Provider
A [Terraform](https://www.terraform.io) provider to manage [Portainer](https://www.portainer.io/) resources via its REST API using Terraform.

It supports provisioning and configuration of Portainer users and will be extended to support other objects such as teams, stacks, endpoints, and access control.

## Requirements
- Terraform v0.13+
- Portainer 2.x with admin API key support enabled
- Go 1.21+ (if building from source)

## Building and Installing
```hcl
make build
```

## Provider Support
| Provider       | Provider Support Status              |
|----------------|--------------------------------------|
| [Terraform](https://registry.terraform.io/providers/portainer/portainer/latest)      | ![Done](https://img.shields.io/badge/status-done-brightgreen)           |
| [OpenTofu](https://search.opentofu.org/provider/portainer/portainer/latest)       | ![Done](https://img.shields.io/badge/status-done-brightgreen) |


## Example Provider Configuration
```hcl
provider "portainer" {
  endpoint = "https://portainer.example.com"
  api_key  = "your-api-key"
  skip_ssl_verify  = true # optional (default value is `false`)
}
```

## Authentication
- Static API key

Static credentials can be provided by adding the `api_key` variables in-line in the Portainer provider block:
> 🔐 **Authentication:** This provider supports only **API keys** via the `X-API-Key` header. JWT tokens curentlly are not supported in this provider.

Usage:

```hcl
provider "portainer" {
  api_key  = "your-api-key"
}
```

### Environment variables
You can provide your configuration via the environment variables representing your minio credentials:

```hcl
$ export PORTAINER_ENDPOINT="https://portainer.example.com"
$ export PORTAINER_API_KEY="your-api-key"
$ export PORTAINER_SKIP_SSL_VERIFY=true
```

## Arguments Reference
| Name       | Type   | Required | Description                                                                 |
|------------|--------|----------|-----------------------------------------------------------------------------|
| `endpoint` | string | ✅ yes   | The URL of the Portainer instance. `/api` will be appended automatically if missing. |
| `api_key`  | string | ✅ yes   | API key used to authenticate requests.                                      |
| `skip_ssl_verify` | boolean | ❌ no | 	Set to `true` to skip TLS certificate verification (useful for self-signed certs). Default: `false` |

## Usage
See our [examples](./docs/resources/) per resources in docs.

## 🧩 Supported Resources
| Resource                     | Documentation                                               | Example                                 | Status                                                             | Terraform Import                                         | E2E Tests                                                 |
|------------------------------|-------------------------------------------------------------|-----------------------------------------|--------------------------------------------------------------------|----------------------------------------------------------|------------------------------------------------------------|
| `portainer_user`             | [user.md](docs/resources/user.md)                          | [example](examples/user/)               | ![Done](https://img.shields.io/badge/status-done-brightgreen)     | ![Yes](https://img.shields.io/badge/import-yes-brightgreen)      | ![Daily](https://img.shields.io/badge/running-daily-blue) |
| `portainer_team`             | [team.md](docs/resources/team.md)                          | [example](examples/team/)               | ![Done](https://img.shields.io/badge/status-done-brightgreen)     | ![Yes](https://img.shields.io/badge/import-yes-brightgreen)      | ![Daily](https://img.shields.io/badge/running-daily-blue) |
| `portainer_team_membership`  | [team_membership.md](docs/resources/team_membership.md)    | [example](examples/team_membership/)    | ![Done](https://img.shields.io/badge/status-done-brightgreen)     | ![Yes](https://img.shields.io/badge/import-yes-brightgreen)      | ![Daily](https://img.shields.io/badge/running-daily-blue) |
| `portainer_environment`      | [environment.md](docs/resources/environment.md)            | [example](examples/environment/)        | ![Done](https://img.shields.io/badge/status-done-brightgreen)     | ![Not yet](https://img.shields.io/badge/import-no-lightgrey)     | ![None](https://img.shields.io/badge/running-daily-blue)  |
| `portainer_tag`              | [tag.md](docs/resources/tag.md)                            | [example](examples/tag/)                | ![Done](https://img.shields.io/badge/status-done-brightgreen)     | ![Yes](https://img.shields.io/badge/import-yes-brightgreen)      | ![Daily](https://img.shields.io/badge/running-daily-blue) |
| `portainer_endpoint_group`   | [endpoint_group.md](docs/resources/endpoint_group.md)      | [example](examples/endpoint_group/)     | ![Done](https://img.shields.io/badge/status-done-brightgreen)     | ![Yes](https://img.shields.io/badge/import-yes-brightgreen)      | ![Daily](https://img.shields.io/badge/running-daily-blue) |
| `portainer_registry`         | [registry.md](docs/resources/registry.md)                  | [example](examples/registry/)           | ![Done](https://img.shields.io/badge/status-done-brightgreen)     | ![Not yet](https://img.shields.io/badge/import-no-lightgrey)     | ![Daily](https://img.shields.io/badge/running-daily-blue) |
| `portainer_backup`           | [backup.md](docs/resources/backup.md)                      | [example](examples/backup/)             | ![Done](https://img.shields.io/badge/status-done-brightgreen)     | ![Not yet](https://img.shields.io/badge/import-no-lightgrey)     | ![Daily](https://img.shields.io/badge/running-daily-blue) |
| `portainer_backup_s3`        | [backup_s3.md](docs/resources/backup_s3.md)                | [example](examples/backup_s3/)          | ![Done](https://img.shields.io/badge/status-done-brightgreen)     | ![Not yet](https://img.shields.io/badge/import-no-lightgrey)     | ![None](https://img.shields.io/badge/running-false-grey)  |
| `portainer_auth`             | [auth.md](docs/resources/auth.md)                          | [example](examples/auth/)               | ![Done](https://img.shields.io/badge/status-done-brightgreen)     | ![Not yet](https://img.shields.io/badge/import-no-lightgrey)     | ![Daily](https://img.shields.io/badge/running-daily-blue) |
| `portainer_edge_group`       | [edge_group.md](docs/resources/edge_group.md)              | [example](examples/edge_group/)         | ![Done](https://img.shields.io/badge/status-done-brightgreen)     | ![Yes](https://img.shields.io/badge/import-yes-brightgreen)      | ![None](https://img.shields.io/badge/running-daily-blue)  |
| `portainer_edge_stack`       | [edge_stack.md](docs/resources/edge_stack.md)              | [example](examples/edge_stack/)         | ![Done](https://img.shields.io/badge/status-done-brightgreen)     | ![Not yet](https://img.shields.io/badge/import-no-lightgrey)     | ![None](https://img.shields.io/badge/running-daily-blue)  |
| `portainer_edge_job`         | [edge_job.md](docs/resources/edge_job.md)                  | [example](examples/edge_job/)           | ![Done](https://img.shields.io/badge/status-done-brightgreen)     | ![Not yet](https://img.shields.io/badge/import-no-lightgrey)     | ![None](https://img.shields.io/badge/running-false-grey)  |
| `portainer_stack`            | [stack.md](docs/resources/stack.md)                        | [example](examples/stack/)              | ![Done](https://img.shields.io/badge/status-done-brightgreen)     | ![Not yet](https://img.shields.io/badge/import-no-lightgrey)     | ![Daily](https://img.shields.io/badge/running-daily-blue) |
| `portainer_custom_template`  | [custom_template.md](docs/resources/custom_template.md)    | [example](examples/custom_template/)    | ![Done](https://img.shields.io/badge/status-done-brightgreen)     | ![Yes](https://img.shields.io/badge/import-yes-brightgreen)      | ![Daily](https://img.shields.io/badge/running-daily-blue) |
| `portainer_container_exec`   | [container_exec.md](docs/resources/container_exec.md)      | [example](examples/container_exec/)     | ![Done](https://img.shields.io/badge/status-done-brightgreen)     | ![Not yet](https://img.shields.io/badge/import-no-lightgrey)     | ![Daily](https://img.shields.io/badge/running-daily-blue) |
| `portainer_docker_network`   | [docker_network.md](docs/resources/docker_network.md)      | [example](examples/docker_network/)     | ![Done](https://img.shields.io/badge/status-done-brightgreen)     | ![Not yet](https://img.shields.io/badge/import-no-lightgrey)     | ![Daily](https://img.shields.io/badge/running-daily-blue) |
| `portainer_docker_image`     | [docker_image.md](docs/resources/docker_image.md)          | [example](examples/docker_image/)       | ![Done](https://img.shields.io/badge/status-done-brightgreen)     | ![Not yet](https://img.shields.io/badge/import-no-lightgrey)     | ![Daily](https://img.shields.io/badge/running-daily-blue) |
| `portainer_docker_volume`    | [docker_volume.md](docs/resources/docker_volume.md)        | [example](examples/docker_volume/)      | ![Done](https://img.shields.io/badge/status-done-brightgreen)     | ![Not yet](https://img.shields.io/badge/import-no-lightgrey)     | ![Daily](https://img.shields.io/badge/running-daily-blue) |
| `portainer_docker_secret`    | [docker_secret.md](docs/resources/docker_secret.md)        | [example](examples/docker_secret/)      | ![Done](https://img.shields.io/badge/status-done-brightgreen)     | ![Not yet](https://img.shields.io/badge/import-no-lightgrey)     | ![Daily](https://img.shields.io/badge/running-daily-blue) |
| `portainer_docker_config`    | [docker_config.md](docs/resources/docker_config.md)        | [example](examples/docker_config/)      | ![Done](https://img.shields.io/badge/status-done-brightgreen)     | ![Not yet](https://img.shields.io/badge/import-no-lightgrey)     | ![Daily](https://img.shields.io/badge/running-daily-blue) |
| `portainer_docker_node`    | [docker_node.md](docs/resources/docker_node.md)        | [example](examples/docker_node/)      | ![Done](https://img.shields.io/badge/status-done-brightgreen)     | ![Not yet](https://img.shields.io/badge/import-no-lightgrey)     | ![Daily](https://img.shields.io/badge/running-false-grey) |
| `portainer_open_amt`         | [open_amt.md](docs/resources/open_amt.md)                  | [example](examples/open_amt/)           | ![Done](https://img.shields.io/badge/status-done-brightgreen)     | ![Not yet](https://img.shields.io/badge/import-no-lightgrey)     | ![None](https://img.shields.io/badge/running-daily-blue)  |
| `portainer_settings`         | [settings.md](docs/resources/settings.md)                  | [example](examples/settings/)           | ![Done](https://img.shields.io/badge/status-done-brightgreen)     | ![Not yet](https://img.shields.io/badge/import-no-lightgrey)     | ![Daily](https://img.shields.io/badge/running-daily-blue) |
| `portainer_endpoint_settings`| [endpoint_settings.md](docs/resources/endpoint_settings.md)| [example](examples/endpoint_settings/)  | ![Done](https://img.shields.io/badge/status-done-brightgreen)     | ![Not yet](https://img.shields.io/badge/import-no-lightgrey)     | ![Daily](https://img.shields.io/badge/running-daily-blue) |
| `portainer_portainer_endpoint_service_update`| [endpoint_service_update.md](docs/resources/endpoint_service_update.md)| [example](examples/endpoint_service_update/)  | ![Done](https://img.shields.io/badge/status-done-brightgreen)     | ![Not yet](https://img.shields.io/badge/import-no-lightgrey)     | ![None](https://img.shields.io/badge/running-daily-blue) |
| `portainer_endpoint_snapshot`| [endpoint_snapshot.md](docs/resources/endpoint_snapshot.md)| [example](examples/endpoint_snapshot/)  | ![Done](https://img.shields.io/badge/status-done-brightgreen)     | ![Not yet](https://img.shields.io/badge/import-no-lightgrey)     | ![Daily](https://img.shields.io/badge/running-daily-blue) |
| `portainer_endpoint_association`| [endpoint_association.md](docs/resources/endpoint_association.md)| [example](examples/endpoint_association/)  | ![Done](https://img.shields.io/badge/status-done-brightgreen)     | ![Not yet](https://img.shields.io/badge/import-no-lightgrey)     | ![None](https://img.shields.io/badge/running-false-grey) |
| `portainer_stack_associate`  | [stack_associate.md](docs/resources/stack_associate.md)    | [example](examples/stack_associate/)    | ![Done](https://img.shields.io/badge/status-done-brightgreen)     | ![Not yet](https://img.shields.io/badge/import-no-lightgrey)     | ![None](https://img.shields.io/badge/running-false-grey) |
| `portainer_ssl`              | [ssl.md](docs/resources/ssl.md)                            | [example](examples/ssl/)                | ![Done](https://img.shields.io/badge/status-done-brightgreen)     | ![Not yet](https://img.shields.io/badge/import-no-lightgrey)     | ![Daily](https://img.shields.io/badge/running-daily-blue) |
| `portainer_webhook`          | [webhook.md](docs/resources/webhook.md)                    | [example](examples/webhook/)            | ![Done](https://img.shields.io/badge/status-done-brightgreen)     | ![Not yet](https://img.shields.io/badge/import-no-lightgrey)     | ![Daily](https://img.shields.io/badge/running-daily-blue) |
| `portainer_webhook_execute`  | [webhook_execute.md](docs/resources/webhook_execute.md)    | [example](examples/webhook_execute/)    | ![Done](https://img.shields.io/badge/status-done-brightgreen)     | ![Not yet](https://img.shields.io/badge/import-no-lightgrey)     | ![None](https://img.shields.io/badge/running-false-grey)  |
| `portainer_resource_control` | [resource_control.md](docs/resources/resource_control.md)| [example](examples/resource_control/)  | ![Done](https://img.shields.io/badge/status-done-brightgreen)     | ![Not yet](https://img.shields.io/badge/import-no-lightgrey)     | ![None](https://img.shields.io/badge/running-false-grey)  |
| `portainer_licenses`         | [licenses.md](docs/resources/licenses.md)                  | [example](examples/licenses/)           | ![Done](https://img.shields.io/badge/status-done-brightgreen)     | ![Not yet](https://img.shields.io/badge/import-no-lightgrey)     | ![None](https://img.shields.io/badge/running-false-grey)  |
| `portainer_cloud_credentials`| [cloud_credentials.md](docs/resources/cloud_credentials.md)| [example](examples/cloud_credentials/)  | ![Done](https://img.shields.io/badge/status-done-brightgreen)     | ![Not yet](https://img.shields.io/badge/import-no-lightgrey)     | ![None](https://img.shields.io/badge/running-false-grey)  |
| `portainer_kubernetes_delete_object`              | [kubernetes_delete_object.md](docs/resources/kubernetes_delete_object.md)              | [example](examples/kubernetes_delete_object/)              | ![Done](https://img.shields.io/badge/status-done-brightgreen)     | ![Not yet](https://img.shields.io/badge/import-no-lightgrey)     | ![None](https://img.shields.io/badge/running-daily-blue)  |
| `portainer_kubernetes_helm`                       | [kubernetes_helm.md](docs/resources/kubernetes_helm.md)                                | [example](examples/kubernetes_helm/)                       | ![Done](https://img.shields.io/badge/status-done-brightgreen)     | ![Not yet](https://img.shields.io/badge/import-no-lightgrey)     | ![Daily](https://img.shields.io/badge/running-daily-blue)  |
| `portainer_kubernetes_ingresscontrollers`         | [kubernetes_ingresscontrollers.md](docs/resources/kubernetes_ingresscontrollers.md)    | [example](examples/kubernetes_ingresscontrollers/)         | ![Done](https://img.shields.io/badge/status-done-brightgreen)     | ![Not yet](https://img.shields.io/badge/import-no-lightgrey)     | ![Daily](https://img.shields.io/badge/running-daily-blue)  |
| `portainer_kubernetes_namespace_ingresscontrollers`| [kubernetes_namespace_ingresscontrollers.md](docs/resources/kubernetes_namespace_ingresscontrollers.md) | [example](examples/kubernetes_namespace_ingresscontrollers/)| ![Done](https://img.shields.io/badge/status-done-brightgreen)     | ![Not yet](https://img.shields.io/badge/import-no-lightgrey)     | ![Daily](https://img.shields.io/badge/running-daily-blue)  |
| `portainer_kubernetes_namespace_system`           | [kubernetes_namespace_system.md](docs/resources/kubernetes_namespace_system.md)        | [example](examples/kubernetes_namespace_system/)           | ![Done](https://img.shields.io/badge/status-done-brightgreen)     | ![Not yet](https://img.shields.io/badge/import-no-lightgrey)     | ![Daily](https://img.shields.io/badge/running-daily-blue)  |
| `portainer_kubernetes_namespace`                  | [kubernetes_namespace.md](docs/resources/kubernetes_namespace.md)                      | [example](examples/kubernetes_namespace/)                  | ![Done](https://img.shields.io/badge/status-done-brightgreen)     | ![Not yet](https://img.shields.io/badge/import-no-lightgrey)     | ![Daily](https://img.shields.io/badge/running-daily-blue)  |
| `portainer_kubernetes_cronjob`                    | [kubernetes_cronjob.md](docs/resources/kubernetes_cronjob.md)                          | [example](examples/kubernetes_cronjob/)                     | ![Done](https://img.shields.io/badge/status-done-brightgreen)     | ![Not yet](https://img.shields.io/badge/import-no-lightgrey)     | ![Daily](https://img.shields.io/badge/running-daily-blue)  |
| `portainer_kubernetes_job`                        | [kubernetes_job.md](docs/resources/kubernetes_job.md)                                  | [example](examples/kubernetes_job/)                         | ![Done](https://img.shields.io/badge/status-done-brightgreen)     | ![Not yet](https://img.shields.io/badge/import-no-lightgrey)     | ![Daily](https://img.shields.io/badge/running-daily-blue)  |
| `portainer_kubernetes_service_accounts`            | [kubernetes_service_account.md](docs/resources/kubernetes_service_account.md)          | [example](examples/kubernetes_serviceaccounts/)             | ![Done](https://img.shields.io/badge/status-done-brightgreen)     | ![Not yet](https://img.shields.io/badge/import-no-lightgrey)     | ![Daily](https://img.shields.io/badge/running-daily-blue)  |
| `portainer_kubernetes_configmaps`                 | [kubernetes_configmaps.md](docs/resources/kubernetes_configmaps.md)                    | [example](examples/kubernetes_configmaps/)                  | ![Done](https://img.shields.io/badge/status-done-brightgreen)     | ![Not yet](https://img.shields.io/badge/import-no-lightgrey)     | ![Daily](https://img.shields.io/badge/running-daily-blue)  |
| `portainer_kubernetes_secret`                     | [kubernetes_secret.md](docs/resources/kubernetes_secret.md)                            | [example](examples/kubernetes_secret/)                      | ![Done](https://img.shields.io/badge/status-done-brightgreen)     | ![Not yet](https://img.shields.io/badge/import-no-lightgrey)     | ![Daily](https://img.shields.io/badge/running-daily-blue)  |
| `portainer_kubernetes_service`                    | [kubernetes_service.md](docs/resources/kubernetes_service.md)                          | [example](examples/kubernetes_service/)                     | ![Done](https://img.shields.io/badge/status-done-brightgreen)     | ![Not yet](https://img.shields.io/badge/import-no-lightgrey)     | ![Daily](https://img.shields.io/badge/running-daily-blue)  |
| `portainer_kubernetes_role`                       | [kubernetes_role.md](docs/resources/kubernetes_role.md)                                | [example](examples/kubernetes_role/)                        | ![Done](https://img.shields.io/badge/status-done-brightgreen)     | ![Not yet](https://img.shields.io/badge/import-no-lightgrey)     | ![Daily](https://img.shields.io/badge/running-daily-blue)  |
| `portainer_kubernetes_rolebinding`                | [kubernetes_rolebinding.md](docs/resources/kubernetes_rolebinding.md)                  | [example](examples/kubernetes_rolebinding/)                 | ![Done](https://img.shields.io/badge/status-done-brightgreen)     | ![Not yet](https://img.shields.io/badge/import-no-lightgrey)     | ![Daily](https://img.shields.io/badge/running-daily-blue)  |
| `portainer_kubernetes_clusterrole`                | [kubernetes_clusterrole.md](docs/resources/kubernetes_clusterrole.md)                  | [example](examples/kubernetes_clusterrole/)                 | ![Done](https://img.shields.io/badge/status-done-brightgreen)     | ![Not yet](https://img.shields.io/badge/import-no-lightgrey)     | ![Daily](https://img.shields.io/badge/running-daily-blue)  |
| `portainer_kubernetes_clusterrolebinding`         | [kubernetes_clusterrolebinding.md](docs/resources/kubernetes_clusterrolebinding.md)    | [example](examples/kubernetes_clusterrolebinding/)          | ![Done](https://img.shields.io/badge/status-done-brightgreen)     | ![Not yet](https://img.shields.io/badge/import-no-lightgrey)     | ![Daily](https://img.shields.io/badge/running-daily-blue)  |
| `portainer_kubernetes_application`                | [kubernetes_application.md](docs/resources/kubernetes_application.md) | [example](examples/kubernetes_application/)    | ![Done](https://img.shields.io/badge/status-done-brightgreen)     | ![Not yet](https://img.shields.io/badge/import-no-lightgrey)     | ![Daily](https://img.shields.io/badge/running-daily-blue)  |
| `portainer_kubernetes_ingress`                    | [kubernetes_ingress.md](docs/resources/kubernetes_ingress.md)                          | [example](examples/kubernetes_ingress/)                     | ![Done](https://img.shields.io/badge/status-done-brightgreen)     | ![Not yet](https://img.shields.io/badge/import-no-lightgrey)     | ![Daily](https://img.shields.io/badge/running-daily-blue)  |
| `portainer_kubernetes_volume`                     | [kubernetes_volume.md](docs/resources/kubernetes_volume.md)                            | [example](examples/kubernetes_volume/)                      | ![Done](https://img.shields.io/badge/status-done-brightgreen)     | ![Not yet](https://img.shields.io/badge/import-no-lightgrey)     | ![Daily](https://img.shields.io/badge/running-daily-blue)  |
| `portainer_kubernetes_storage`                    | [kubernetes_storage.md](docs/resources/kubernetes_storage.md)                          | [example](examples/kubernetes_storage/)                     | ![Done](https://img.shields.io/badge/status-done-brightgreen)     | ![Not yet](https://img.shields.io/badge/import-no-lightgrey)     | ![Daily](https://img.shields.io/badge/running-daily-blue)  |

---

### 💡 Missing a resource?
Is there a Portainer resource you'd like to see supported?

👉 [Open an issue](https://github.com/portainer/terraform-provider-portainer/issues/new?template=feature_request.md) and we’ll consider it for implementation — or even better, submit a [Pull Request](https://github.com/portainer/terraform-provider-portainer/pulls) to contribute directly!

📘 See [CONTRIBUTING.md](./.github/CONTRIBUTING.md) for guidelines.

## 💬 Community & Feedback
Have questions, suggestions or want to contribute ideas?  
Join the **Portainer Community Slack** and hop into the [`#portainer-terraform`](https://app.slack.com/client/T2AGA35A4/C08NHK6PLUT) channel!

Want to report issues, submit pull requests or browse the source code?  
Check out the [GitHub Repository](https://github.com/portainer/terraform-provider-portainer) for this provider.

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

## ✅ Daily End-to-End Testing
To ensure maximum reliability and functionality of this provider, **automated end-to-end tests are executed every day** via GitHub Actions.

These tests run against a real Portainer instance (started using docker compose) and validate the majority of supported resources using real Terraform plans and applies.

> 💡 This helps catch regressions early and ensures the provider remains fully operational and compatible with the Portainer API.

### 🔄 Workflows
The project uses GitHub Actions to automate validation and testing of the provider.

- Validate and lint documentation files (`README.md` and `docs/`)
- Initialize, test and check the Portainer provider with **Terraform** and **OpenTofu**
- Publish the new version of the Portainer Terraform provider to Terraform Registry
- Run daily **E2E Terraform tests** against a live Portainer instance spun up via Docker Compose (`make up`) at **07:00 UTC**

### 🧪 Localy Testing
To test the provider locally, start the Portainer Web UI using Docker Compose:
```sh
make up
```
Then open `http://localhost:9000` in your browser.

### 🔐 Predefined Test Credentials for Login (use also E2E tests)
Thanks to the `portainer_data` directory included in this repository, a test user and token are preloaded when you launch the local Portainer instance:

| **Field**    | **Value**                                                                  |
|--------------|----------------------------------------------------------------------------|
| Username     | `admin`                                                                    |
| Password     | `password123456789`                                                        |
| API Token    | `ptr_xrP7XWqfZEOoaCJRu5c8qKaWuDtVc2Zb07Q5g22YpS8=`                         |

You can now apply your Terraform templates and observe changes live in the UI.

### ☸️ Testing Kubernetes Resources Locally
If you want to test Kubernetes-related resources, you can spin up a local Kubernetes cluster with [k3d](https://k3d.io/), deploy the Portainer Agent into it, and connect Portainer to that environment:

```sh
make install-k3d             # Install k3d CLI
make k3d-up                  # Create a local k3d cluster
make k8s-deploy-agent        # Deploy Portainer Agent into Kubernetes
make k3d-connect-portainer   # Connect Portainer container to the k3d network
make k3d-export-ip           # Export Kubernetes IP into terraform.tfvars
```

Then you can apply your Kubernetes environemnt from directory `e2e-tests/environment` run by:

```sh
cd e2e-tests/environment
terraform init
terraform apply
```

and than Kubernetes-related Terraform templates under e2e-tests/kubernetes* (or a similar directory):

```sh
cd e2e-tests/kubernetes*
terraform init
terraform apply
```

### Testing a new version of the Portainer provider
After making changes to the provider source code, follow these steps:
Build the provider binary:
```sh
make build
```
Install the binary into the local Terraform plugin directory:
```sh
make install-plugin
```
Update your main.tf to use the local provider source
Add the following to your Terraform configuration:
```sh
terraform {
  required_providers {
    portainer = {
      source  = "localdomain/local/portainer"
    }
  }
}
```
Now you're ready to test your provider against the local Portainer instance.

## Roadmap
See the [open issues](https://github.com/portainer/terraform-provider-portainer/issues) for a list of proposed features (and known issues). See [CONTRIBUTING](./.github/CONTRIBUTING.md) for more information.

## License
This module is 100% Open Source and is distributed under the MIT License.  
See the [LICENSE](https://github.com/portainer/terraform-provider-portainer/blob/main/LICENSE) file for more information.


## Acknowledgements
- [HashiCorp Terraform](https://www.hashicorp.com/products/terraform)
- [Portainer](https://portainer.io)
- [OpenTofu](https://opentofu.org/)
- [Docker](https://www.docker.com/)
