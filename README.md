<p align="center">
  <a href="https://registry.terraform.io/providers/portainer/portainer/latest/docs">
    <img src="https://camo.githubusercontent.com/cdda8928975712cecce7be8b6a1506e3b327b1643cd3391dcf40515e25b54f73/68747470733a2f2f7777772e6461746f636d732d6173736574732e636f6d2f323838352f313733313337333331302d7465727261666f726d5f77686974652e737667" alt="Terraform Logo" width="200">
  </a>
  &nbsp;&nbsp;&nbsp;
  <a href="https://github.com/portainer/terraform-provider-portainer">
    <img src="https://raw.githubusercontent.com/portainer/portainer/refs/heads/develop/app/assets/images/portainer-github-banner.png" alt="portainer-provider-terraform" width="200">
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
A Terraform provider to manage [Portainer](https://www.portainer.io/) resources via its REST API using Terraform.

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
| Provider                                                                             | Provider Support Status   |
|--------------------------------------------------------------------------------------|---------------------------|
| [Terraform](https://registry.terraform.io/providers/portainer/portainer/latest)      | ✅                        |
| [OpenTofu](https://search.opentofu.org/provider/portainer/portainer/latest)          | ✅                        |


## Example Provider Configuration
```hcl
provider "portainer" {
  endpoint = "https://portainer.example.com"

  # Option 1: API key authentication
  api_key  = "your-api-key"

  # Option 2: Username/password authentication (generates JWT token internally)
  # api_user     = "admin"
  # api_password = "your-password"

  skip_ssl_verify  = true # optional (default value is `false`)
}
```

## Authentication
The Portainer Terraform provider supports two authentication methods:
1. **API Key** (via `X-API-Key` header)
2. **Username & Password** (via `/api/auth` → JWT token internally used)

Only one method is required – if both are provided, `api_key` takes precedence.

#### Usage – API Key:

```hcl
provider "portainer" {
  api_key  = "your-api-key"
}
```

#### Usage – Username & Password:

```hcl
provider "portainer" {
  api_user     = "admin"
  api_password = "your-password"
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
| Name              | Type    | Required | Description                                                                                         |
| ----------------- | ------- | -------- | ----------------------------------------------------------------------------------------------------|
| `endpoint`        | string  | ✅ yes   | URL of the Portainer instance. `/api` will be appended automatically if missing.                    |
| `api_key`         | string  | ❌ no    | API key for authentication. Mutually exclusive with `api_user` and `api_password`.                  |
| `api_user`        | string  | ❌ no    | Username for authentication (must be used with `api_password`). Mutually exclusive with `api_key`.  |
| `api_password`    | string  | ❌ no    | Password for authentication (must be used with `api_user`). Mutually exclusive with `api_key`.      |
| `skip_ssl_verify` | boolean | ❌ no    | Skip TLS certificate verification (useful for self-signed certs). Default: `false`.                 |


## Usage
See our [examples](./docs/resources/) per resources in docs.

## 🧩 Supported Resources
| Resource                                   | Documentation                                                                                  | Example                                              | Status | Terraform Import / Create => Update | E2E Tests |
|--------------------------------------------|------------------------------------------------------------------------------------------------|------------------------------------------------------|--------|-------------------------------------|-----------|
| `portainer_user`                           | [user.md](docs/resources/user.md)                                                              | [example](examples/user/)                            | ✅     | ✅ / ✅                             | ✅        |
| `portainer_team`                           | [team.md](docs/resources/team.md)                                                              | [example](examples/team/)                            | ✅     | ✅ / ✅                             | ✅        |
| `portainer_team_membership`                | [team_membership.md](docs/resources/team_membership.md)                                        | [example](examples/team_membership/)                 | ✅     | ✅ / ❌                             | ✅        |
| `portainer_environment`                    | [environment.md](docs/resources/environment.md)                                                | [example](examples/environment/)                     | ✅     | ✅ / ❌                             | ❌        |
| `portainer_tag`                            | [tag.md](docs/resources/tag.md)                                                                | [example](examples/tag/)                             | ✅     | ✅ / ✅                             | ✅        |
| `portainer_endpoint_group`                 | [endpoint_group.md](docs/resources/endpoint_group.md)                                          | [example](examples/endpoint_group/)                  | ✅     | ✅ / ✅                             | ✅        |
| `portainer_registry`                       | [registry.md](docs/resources/registry.md)                                                      | [example](examples/registry/)                        | ✅     | ✅ / ✅                             | ✅        |
| `portainer_backup`                         | [backup.md](docs/resources/backup.md)                                                          | [example](examples/backup/)                          | ✅     | ❌ / ❌                             | ✅        |
| `portainer_backup_s3`                      | [backup_s3.md](docs/resources/backup_s3.md)                                                    | [example](examples/backup_s3/)                       | ✅     | ❌ / ❌                             | ❌        |
| `portainer_auth`                           | [auth.md](docs/resources/auth.md)                                                              | [example](examples/auth/)                            | ✅     | ❌ / ❌                             | ✅        |
| `portainer_edge_group`                     | [edge_group.md](docs/resources/edge_group.md)                                                  | [example](examples/edge_group/)                      | ✅     | ✅ / ✅                             | ❌        |
| `portainer_edge_stack`                     | [edge_stack.md](docs/resources/edge_stack.md)                                                  | [example](examples/edge_stack/)                      | ✅     | ✅ / ✅                             | ❌        |
| `portainer_edge_job`                       | [edge_job.md](docs/resources/edge_job.md)                                                      | [example](examples/edge_job/)                        | ✅     | ✅ / ✅                             | ❌        |
| `portainer_endpoints_edge_generate_key`    | [endpoints_edge_generate_key.md](docs/resources/endpoints_edge_generate_key.md)                | [example](examples/endpoints_edge_generate_key/)     | ✅     | ❌ / ❌                             | ❌        |
| `portainer_edge_configurations`            | [edge_configurations.md](docs/resources/edge_configurations.md)                                | [example](examples/edge_configurations/)             | ✅     | ✅ / ❌                             | ❌        |
| `portainer_edge_update_schedules`          | [edge_update_schedules.md](docs/resources/edge_update_schedules.md)                            | [example](examples/edge_update_schedules/)           | ✅     | ✅ / ❌                             | ❌        |
| `portainer_stack`                          | [stack.md](docs/resources/stack.md)                                                            | [example](examples/stack/)                           | ✅     | ✅ / ✅                             | ✅        |
| `portainer_custom_template`                | [custom_template.md](docs/resources/custom_template.md)                                        | [example](examples/custom_template/)                 | ✅     | ✅ / ✅                             | ✅        |
| `portainer_container_exec`                 | [container_exec.md](docs/resources/container_exec.md)                                          | [example](examples/container_exec/)                  | ✅     | ❌ / ❌                             | ✅        |
| `portainer_deploy`                         | [deploy.md](docs/resources/deploy.md)                                                          | [example](examples/deployment/)                      | ✅     | ❌ / ❌                             | ✅        |
| `portainer_check`                          | [check.md](docs/resources/check.md)                                                            | [example](examples/deployment/)                      | ✅     | ❌ / ❌                             | ✅        |
| `portainer_docker_network`                 | [docker_network.md](docs/resources/docker_network.md)                                          | [example](examples/docker_network/)                  | ✅     | ✅ / ❌                             | ✅        |
| `portainer_docker_plugin`                  | [docker_plugin.md](docs/resources/docker_plugin.md)                                            | [example](examples/docker_plugin/)                   | ✅     | ✅ / ❌                             | ✅        |
| `portainer_docker_image`                   | [docker_image.md](docs/resources/docker_image.md)                                              | [example](examples/docker_image/)                    | ✅     | ❌ / ❌                             | ✅        |
| `portainer_docker_volume`                  | [docker_volume.md](docs/resources/docker_volume.md)                                            | [example](examples/docker_volume/)                   | ✅     | ✅ / ❌                             | ✅        |
| `portainer_docker_secret`                  | [docker_secret.md](docs/resources/docker_secret.md)                                            | [example](examples/docker_secret/)                   | ✅     | ✅ / ✅                             | ✅        |
| `portainer_docker_config`                  | [docker_config.md](docs/resources/docker_config.md)                                            | [example](examples/docker_config/)                   | ✅     | ✅ / ✅                             | ✅        |
| `portainer_docker_node`                    | [docker_node.md](docs/resources/docker_node.md)                                                | [example](examples/docker_node/)                     | ✅     | ❌ / ❌                             | ❌        |
| `portainer_open_amt`                       | [open_amt.md](docs/resources/open_amt.md)                                                      | [example](examples/open_amt/)                        | ✅     | ❌ / ❌                             | ❌        |
| `portainer_open_amt_activate`              | [open_amt_activate.md](docs/resources/open_amt_activate.md)                                    | [example](examples/open_amt_activate/)               | ✅     | ❌ / ❌                             | ❌        |
| `portainer_open_amt_devices_action`        | [open_amt_devices_action.md](docs/resources/open_amt_devices_action.md)                        | [example](examples/open_amt_devices_action/)         | ✅     | ❌ / ❌                             | ❌        |
| `portainer_open_amt_devices_features`      | [open_amt_devices_features.md](docs/resources/open_amt_devices_features.md)                    | [example](examples/open_amt_devices_features/)       | ✅     | ❌ / ❌                             | ❌        |
| `portainer_settings`                       | [settings.md](docs/resources/settings.md)                                                      | [example](examples/settings/)                        | ✅     | ✅ / ❌                             | ✅        |
| `portainer_settings_experimental`          | [settings_experimental.md](docs/resources/settings_experimental.md)                            | [example](examples/settings_experimental/)           | ✅     | ✅ / ❌                             | ❌        |
| `portainer_endpoint_settings`              | [endpoint_settings.md](docs/resources/endpoint_settings.md)                                    | [example](examples/endpoint_settings/)               | ✅     | ❌ / ❌                             | ✅        |
| `portainer_portainer_endpoint_service_update` | [endpoint_service_update.md](docs/resources/endpoint_service_update.md)                     | [example](examples/endpoint_service_update/)         | ✅     | ❌ / ❌                             | ❌        |
| `portainer_endpoint_snapshot`              | [endpoint_snapshot.md](docs/resources/endpoint_snapshot.md)                                    | [example](examples/endpoint_snapshot/)               | ✅     | ❌ / ❌                             | ✅        |
| `portainer_endpoint_association`           | [endpoint_association.md](docs/resources/endpoint_association.md)                              | [example](examples/endpoint_association/)            | ✅     | ❌ / ❌                             | ❌        |
| `portainer_stack_associate`                | [stack_associate.md](docs/resources/stack_associate.md)                                        | [example](examples/stack_associate/)                 | ✅     | ❌ / ❌                             | ❌        |
| `portainer_ssl`                            | [ssl.md](docs/resources/ssl.md)                                                                | [example](examples/ssl/)                             | ✅     | ✅ / ❌                             | ✅        |
| `portainer_tls`                            | [tls.md](docs/resources/tls.md)                                                                | [example](examples/tls/)                             | ✅     | ❌ / ❌                             | ❌        |
| `portainer_webhook`                        | [webhook.md](docs/resources/webhook.md)                                                        | [example](examples/webhook/)                         | ✅     | ❌ / ❌                             | ✅        |
| `portainer_stack_webhook`                  | [stack_webhook.md](docs/resources/stack_webhook.md)                                            | [example](examples/stack_webhook/)                   | ✅     | ❌ / ❌                             | ❌        |
| `portainer_edge_stack_webhook`             | [edge_stack_webhook.md](docs/resources/edge_stack_webhook.md)                                  | [example](examples/edge_stack_webhook/)              | ✅     | ❌ / ❌                             | ❌        |
| `portainer_webhook_execute`                | [webhook_execute.md](docs/resources/webhook_execute.md)                                        | [example](examples/webhook_execute/)                 | ✅     | ❌ / ❌                             | ❌        |
| `portainer_resource_control`               | [resource_control.md](docs/resources/resource_control.md)                                      | [example](examples/resource_control/)                | ✅     | ❌ / ❌                             | ❌        |
| `portainer_licenses`                       | [licenses.md](docs/resources/licenses.md)                                                      | [example](examples/licenses/)                        | ✅     | ✅ / ❌                             | ❌        |
| `portainer_cloud_credentials`              | [cloud_credentials.md](docs/resources/cloud_credentials.md)                                    | [example](examples/cloud_credentials/)               | ✅     | ✅ / ❌                             | ❌        |
| `portainer_cloud_provider_provision`       | [cloud_provider_provision.md](docs/resources/cloud_provider_provision.md)                      | [example](examples/cloud_provider_provision/)        | ✅     | ❌ / ❌                             | ❌        |
| `portainer_compose_convert`                | [compose_convert.md](docs/resources/compose_convert.md)                                        | [example](examples/compose_convert/)                 | ✅     | ❌ / ❌                             | ✅        |
| `portainer_chat`                           | [chat.md](docs/resources/chat.md)                                                              | [example](examples/chat/)                            | ✅     | ❌ / ❌                             | ❌        |
| `portainer_support_debug_log`              | [support_debug_log.md](docs/resources/support_debug_log.md)                                    | [example](examples/support_debug_log/)               | ✅     | ✅ / ❌                             | ❌        |
| `portainer_sshkeygen`                      | [sshkeygen.md](docs/resources/sshkeygen.md)                                                    | [example](examples/sshkeygen/)                       | ✅     | ❌ / ❌                             | ❌        |
| `portainer_kubernetes_delete_object`       | [kubernetes_delete_object.md](docs/resources/kubernetes_delete_object.md)                      | [example](examples/kubernetes_delete_object/)        | ✅     | ❌ / ❌                             | ❌        |
| `portainer_kubernetes_helm`                | [kubernetes_helm.md](docs/resources/kubernetes_helm.md)                                        | [example](examples/kubernetes_helm/)                 | ✅     | ❌ / ❌                             | ✅        |
| `portainer_kubernetes_ingresscontrollers`  | [kubernetes_ingresscontrollers.md](docs/resources/kubernetes_ingresscontrollers.md)            | [example](examples/kubernetes_ingresscontrollers/)   | ✅     | ❌ / ❌                             | ✅        |
| `portainer_kubernetes_namespace_ingresscontrollers` | [kubernetes_namespace_ingresscontrollers.md](docs/resources/kubernetes_namespace_ingresscontrollers.md) | [example](examples/kubernetes_namespace_ingresscontrollers/)| ✅ | ❌ / ❌        | ✅        |
| `portainer_kubernetes_namespace_system`    | [kubernetes_namespace_system.md](docs/resources/kubernetes_namespace_system.md)                | [example](examples/kubernetes_namespace_system/)     | ✅     | ❌ / ❌                             | ✅        |
| `portainer_kubernetes_namespace`           | [kubernetes_namespace.md](docs/resources/kubernetes_namespace.md)                              | [example](examples/kubernetes_namespace/)            | ✅     | ❌ / ❌                             | ✅        |
| `portainer_kubernetes_namespace_access`    | [kubernetes_namespace_access.md](docs/resources/kubernetes_namespace_access.md)                | [example](examples/kubernetes_namespace_access/)     | ✅     | ❌ / ❌                             | ❌        |
| `portainer_kubernetes_cronjob`             | [kubernetes_cronjob.md](docs/resources/kubernetes_cronjob.md)                                  | [example](examples/kubernetes_cronjob/)              | ✅     | ❌ / ❌                             | ✅        |
| `portainer_kubernetes_job`                 | [kubernetes_job.md](docs/resources/kubernetes_job.md)                                          | [example](examples/kubernetes_job/)                  | ✅     | ❌ / ❌                             | ✅        |
| `portainer_kubernetes_service_accounts`    | [kubernetes_service_account.md](docs/resources/kubernetes_service_account.md)                  | [example](examples/kubernetes_serviceaccounts/)      | ✅     | ❌ / ❌                             | ✅        |
| `portainer_kubernetes_configmaps`          | [kubernetes_configmaps.md](docs/resources/kubernetes_configmaps.md)                            | [example](examples/kubernetes_configmaps/)           | ✅     | ❌ / ❌                             | ✅        |
| `portainer_kubernetes_secret`              | [kubernetes_secret.md](docs/resources/kubernetes_secret.md)                                    | [example](examples/kubernetes_secret/)               | ✅     | ❌ / ❌                             | ✅        |
| `portainer_kubernetes_service`             | [kubernetes_service.md](docs/resources/kubernetes_service.md)                                  | [example](examples/kubernetes_service/)              | ✅     | ❌ / ❌                             | ✅        |
| `portainer_kubernetes_role`                | [kubernetes_role.md](docs/resources/kubernetes_role.md)                                        | [example](examples/kubernetes_role/)                 | ✅     | ❌ / ❌                             | ✅        |
| `portainer_kubernetes_rolebinding`         | [kubernetes_rolebinding.md](docs/resources/kubernetes_rolebinding.md)                          | [example](examples/kubernetes_rolebinding/)          | ✅     | ❌ / ❌                             | ✅        |
| `portainer_kubernetes_clusterrole`         | [kubernetes_clusterrole.md](docs/resources/kubernetes_clusterrole.md)                          | [example](examples/kubernetes_clusterrole/)          | ✅     | ❌ / ❌                             | ✅        |
| `portainer_kubernetes_clusterrolebinding`  | [kubernetes_clusterrolebinding.md](docs/resources/kubernetes_clusterrolebinding.md)            | [example](examples/kubernetes_clusterrolebinding/)   | ✅     | ❌ / ❌                             | ✅        |
| `portainer_kubernetes_application`         | [kubernetes_application.md](docs/resources/kubernetes_application.md)                          | [example](examples/kubernetes_application/)          | ✅     | ❌ / ❌                             | ✅        |
| `portainer_kubernetes_ingress`             | [kubernetes_ingress.md](docs/resources/kubernetes_ingress.md)                                  | [example](examples/kubernetes_ingress/)              | ✅     | ❌ / ❌                             | ✅        |
| `portainer_kubernetes_volume`              | [kubernetes_volume.md](docs/resources/kubernetes_volume.md)                                    | [example](examples/kubernetes_volume/)               | ✅     | ❌ / ❌                             | ✅        |
| `portainer_kubernetes_storage`             | [kubernetes_storage.md](docs/resources/kubernetes_storage.md)                                  | [example](examples/kubernetes_storage/)              | ✅     | ❌ / ❌                             | ✅        |



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
See full documentation: [docs/resources/compose_convert.md](docs/resources/compose_convert.md)

#### ℹ️ Note on Create ⇒ Update Behavior

Some resources support a "Create-or-Update" mechanism, when this behavior is implemented, it means:
> During the initial terraform apply, if an entity with the given name already exists, the resource will detect it and perform an update instead of attempting to create a duplicate => this is achieved by filtering existing entities by name before creation.
- This avoids the need for manual terraform import without having to have a terraform tfstate file or cleanup of existing resources in Portainer.
- It's especially useful during migrations, initial setup, or when applying configuration into environments with pre-existing state.

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
- HashiCorp Terraform
- [Portainer](https://portainer.io)
- [OpenTofu](https://opentofu.org/)
- [Docker](https://www.docker.com/)
