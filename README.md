<p align="center">
  <a href="https://registry.terraform.io/providers/portainer/portainer/latest/docs">
    <img src="https://www.datocms-assets.com/2885/1731373310-terraform_white.svg" alt="Terraform Logo" width="200">
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
- Go 1.27+ (if building from source)

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

  # Optional: add custom headers to ALL requests (e.g. Cloudflare Access / auth proxy)
  # custom_headers = {
  #   "CF-Access-Client-Id"     = "..."
  #   "CF-Access-Client-Secret" = "..."
  # }
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

### HTTP Proxy Support
The provider honors the standard `HTTP_PROXY`, `HTTPS_PROXY` and `NO_PROXY` environment variables (same behavior as Go's `http.DefaultTransport`). This lets you reach a Portainer endpoint that is only accessible through an HTTP(S) proxy – no provider configuration is required.

```hcl
$ export HTTPS_PROXY="http://proxy.example.com:8080"
$ export NO_PROXY="localhost,127.0.0.1,.internal.example.com"
```

> These variables are a no-op when unset, so existing setups are unaffected. Lowercase variants (`http_proxy`, `https_proxy`, `no_proxy`) are also recognized.

## Arguments Reference
| Name              | Type    | Required | Description                                                                                         |
| ----------------- | ------- | -------- | ----------------------------------------------------------------------------------------------------|
| `endpoint`        | string  | ✅ yes   | URL of the Portainer instance. `/api` will be appended automatically if missing.                    |
| `api_key`         | string  | ❌ no    | API key for authentication. Mutually exclusive with `api_user` and `api_password`.                  |
| `api_user`        | string  | ❌ no    | Username for authentication (must be used with `api_password`). Mutually exclusive with `api_key`.  |
| `api_password`    | string  | ❌ no    | Password for authentication (must be used with `api_user`). Mutually exclusive with `api_key`.      |
| `skip_ssl_verify` | boolean | ❌ no    | Skip TLS certificate verification (useful for self-signed certs). Default: `false`.                 |
| `custom_headers`  | map(string) | ❌ no | Custom headers added to all requests (e.g. Cloudflare Access / security proxy headers).            |


## Usage
See our [examples](./docs/resources/) per resources in docs.

## 🧩 Supported Resources
| Resource                                   | Documentation                                                                                  | Example                                              | Status | Terraform Import / Create => Update | E2E Tests |
|--------------------------------------------|------------------------------------------------------------------------------------------------|------------------------------------------------------|--------|-------------------------------------|-----------|
| `portainer_user`                           | [user.md](docs/resources/user.md)                                                              | [example](examples/user/)                            | ✅     | ✅ / ✅                             | ✅        |
| `portainer_user_admin`                     | [user_admin.md](docs/resources/user_admin.md)                                                  | [example](examples/user_admin/)                      | ✅     | ❌ / ❌                             | ✅        |
| `portainer_team`                           | [team.md](docs/resources/team.md)                                                              | [example](examples/team/)                            | ✅     | ✅ / ✅                             | ✅        |
| `portainer_team_membership`                | [team_membership.md](docs/resources/team_membership.md)                                        | [example](examples/team_membership/)                 | ✅     | ✅ / ❌                             | ✅        |
| `portainer_environment`                    | [environment.md](docs/resources/environment.md)                                                | [example](examples/environment/)                     | ✅     | ✅ / ❌                             | ❌        |
| `portainer_tag`                            | [tag.md](docs/resources/tag.md)                                                                | [example](examples/tag/)                             | ✅     | ✅ / ✅                             | ✅        |
| `portainer_endpoint_group`                 | [endpoint_group.md](docs/resources/endpoint_group.md)                                          | [example](examples/endpoint_group/)                  | ✅     | ✅ / ✅                             | ✅        |
| `portainer_endpoint_group_access`          | [endpoint_group_access.md](docs/resources/endpoint_group_access.md)                            | [example](examples/endpoint_group_access/)           | ✅     | ❌ / ❌                             | ✅        |
| `portainer_registry`                       | [registry.md](docs/resources/registry.md)                                                      | [example](examples/registry/)                        | ✅     | ✅ / ✅                             | ✅        |
| `portainer_registry_access`                | [registry_access.md](docs/resources/registry_access.md)                                        | [example](examples/registry/)                        | ✅     | ✅ / ✅                             | ✅        |
| `portainer_backup`                         | [backup.md](docs/resources/backup.md)                                                          | [example](examples/backup/)                          | ✅     | ❌ / ❌                             | ✅        |
| `portainer_backup_s3`                      | [backup_s3.md](docs/resources/backup_s3.md)                                                    | [example](examples/backup_s3/)                       | ✅     | ❌ / ❌                             | ❌        |
| `portainer_backup_azure_settings` | [backup_azure_settings.md](docs/resources/backup_azure_settings.md) | [example](examples/backup_azure_settings/) | ✅     | ❌ / ❌                             | ❌        |
| `portainer_backup_azure_execute` | [backup_azure_execute.md](docs/resources/backup_azure_execute.md) | [example](examples/backup_azure_execute/) | ✅     | ❌ / ❌                             | ❌        |
| `portainer_backup_azure_restore` | [backup_azure_restore.md](docs/resources/backup_azure_restore.md) | [example](examples/backup_azure_restore/) | ✅     | ❌ / ❌                             | ❌        |
| `portainer_backup_local_settings` | [backup_local_settings.md](docs/resources/backup_local_settings.md) | [example](examples/backup_local_settings/) | ✅     | ❌ / ❌                             | ❌        |
| `portainer_backup_local_run` | [backup_local_run.md](docs/resources/backup_local_run.md) | [example](examples/backup_local_run/) | ✅     | ❌ / ❌                             | ❌        |
| `portainer_backup_s3_restore` | [backup_s3_restore.md](docs/resources/backup_s3_restore.md) | [example](examples/backup_s3_restore/) | ✅     | ❌ / ❌                             | ❌        |
| `portainer_addon` | [addon.md](docs/resources/addon.md) | [example](examples/addon/) | ✅     | ✅ / ✅                             | ❌        |
| `portainer_addon_access` | [addon_access.md](docs/resources/addon_access.md) | [example](examples/addon_access/) | ✅     | ✅ / ✅                             | ❌        |
| `portainer_addon_config` | [addon_config.md](docs/resources/addon_config.md) | [example](examples/addon_config/) | ✅     | ✅ / ✅                             | ❌        |
| `portainer_addon_repair` | [addon_repair.md](docs/resources/addon_repair.md) | [example](examples/addon_repair/) | ✅     | ❌ / ❌                             | ❌        |
| `portainer_settings_default_registry` | [settings_default_registry.md](docs/resources/settings_default_registry.md) | [example](examples/settings_default_registry/) | ✅     | ✅ / ✅                             | ❌        |
| `portainer_settings_additional_functionality` | [settings_additional_functionality.md](docs/resources/settings_additional_functionality.md) | [example](examples/settings_additional_functionality/) | ✅     | ✅ / ✅                             | ❌        |
| `portainer_endpoint_trust` | [endpoint_trust.md](docs/resources/endpoint_trust.md) | [example](examples/endpoint_trust/) | ✅     | ✅ / ❌                             | ❌        |
| `portainer_alerting_rule_groups` | [alerting_rule_groups.md](docs/resources/alerting_rule_groups.md) | [example](examples/alerting_rule_groups/) | ✅     | ✅ / ✅                             | ❌        |
| `portainer_alerting_rule_tiers` | [alerting_rule_tiers.md](docs/resources/alerting_rule_tiers.md) | [example](examples/alerting_rule_tiers/) | ✅     | ✅ / ✅                             | ❌        |
| `portainer_ssrf_allowlist` | [ssrf_allowlist.md](docs/resources/ssrf_allowlist.md) | [example](examples/ssrf_allowlist/) | ✅     | ✅ / ✅                             | ❌        |
| `portainer_omni_cluster` | [omni_cluster.md](docs/resources/omni_cluster.md) | [example](examples/omni_cluster/) | ✅     | ✅ / ✅                             | ❌        |
| `portainer_omni_node_reboot` | [omni_node_reboot.md](docs/resources/omni_node_reboot.md) | [example](examples/omni_node_reboot/) | ✅     | ❌ / ❌                             | ❌        |
| `portainer_kubernetes_pod_security_rule` | [kubernetes_pod_security_rule.md](docs/resources/kubernetes_pod_security_rule.md) | [example](examples/kubernetes_pod_security_rule/) | ✅     | ✅ / ✅                             | ❌        |
| `portainer_gitops_workflow` | [gitops_workflow.md](docs/resources/gitops_workflow.md) | [example](examples/gitops_workflow/) | ✅     | ✅ / ✅                             | ❌        |
| `portainer_kubernetes_cluster_upgrade` | [kubernetes_cluster_upgrade.md](docs/resources/kubernetes_cluster_upgrade.md) | [example](examples/kubernetes_cluster_upgrade/) | ✅     | ❌ / ❌                             | ❌        |
| `portainer_user_memberships_sync` | [user_memberships_sync.md](docs/resources/user_memberships_sync.md) | [example](examples/user_memberships_sync/) | ✅     | ❌ / ❌                             | ❌        |
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
| `portainer_resource_control`               | [resource_control.md](docs/resources/resource_control.md)                                      | [example](examples/resource_control/)                | ✅     | ❌ / ❌                             | ✅        |
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
| `portainer_kubernetes_serviceaccounts`     | [kubernetes_serviceaccounts.md](docs/resources/kubernetes_serviceaccounts.md)                  | [example](examples/kubernetes_serviceaccounts/)      | ✅     | ❌ / ❌                             | ✅        |
| `portainer_kubernetes_configmaps`          | [kubernetes_configmaps.md](docs/resources/kubernetes_configmaps.md)                            | [example](examples/kubernetes_configmaps/)           | ✅     | ❌ / ❌                             | ✅        |
| `portainer_kubernetes_secret`              | [kubernetes_secret.md](docs/resources/kubernetes_secret.md)                                    | [example](examples/kubernetes_secret/)               | ✅     | ❌ / ❌                             | ✅        |
| `portainer_kubernetes_service`             | [kubernetes_service.md](docs/resources/kubernetes_service.md)                                  | [example](examples/kubernetes_service/)              | ✅     | ❌ / ❌                             | ✅        |
| `portainer_kubernetes_role`                | [kubernetes_role.md](docs/resources/kubernetes_role.md)                                        | [example](examples/kubernetes_role/)                 | ✅     | ❌ / ❌                             | ✅        |
| `portainer_kubernetes_rolebinding`         | [kubernetes_rolebinding.md](docs/resources/kubernetes_rolebinding.md)                          | [example](examples/kubernetes_rolebinding/)          | ✅     | ❌ / ❌                             | ✅        |
| `portainer_kubernetes_clusterrole`         | [kubernetes_clusterrole.md](docs/resources/kubernetes_clusterrole.md)                          | [example](examples/kubernetes_clusterrole/)          | ✅     | ❌ / ❌                             | ✅        |
| `portainer_kubernetes_clusterrolebinding`  | [kubernetes_clusterrolebinding.md](docs/resources/kubernetes_clusterrolebinding.md)            | [example](examples/kubernetes_clusterrolebinding/)   | ✅     | ❌ / ❌                             | ✅        |
| `portainer_kubernetes_application`         | [kubernetes_application.md](docs/resources/kubernetes_application.md)                          | [example](examples/kubernetes_application/)          | ✅     | ❌ / ❌                             | ✅        |
| `portainer_kubernetes_ingresses`           | [kubernetes_ingresses.md](docs/resources/kubernetes_ingresses.md)                              | [example](examples/kubernetes_ingresses/)            | ✅     | ❌ / ❌                             | ✅        |
| `portainer_kubernetes_volume`              | [kubernetes_volume.md](docs/resources/kubernetes_volume.md)                                    | [example](examples/kubernetes_volume/)               | ✅     | ❌ / ❌                             | ✅        |
| `portainer_kubernetes_node_drain`          | [kubernetes_node_drain.md](docs/resources/kubernetes_node_drain.md)                            | [example](examples/kubernetes_node_drain/)           | ✅     | ❌ / ❌                             | ❌        |
| `portainer_kubernetes_deployment_scale`    | [kubernetes_deployment_scale.md](docs/resources/kubernetes_deployment_scale.md)                | [example](examples/kubernetes_deployment_scale/)     | ✅     | ❌ / ✅                             | ✅        |
| `portainer_kubernetes_deployment_rollback` | [kubernetes_deployment_rollback.md](docs/resources/kubernetes_deployment_rollback.md)          | [example](examples/kubernetes_deployment_rollback/)  | ✅     | ❌ / ❌                             | ❌        |
| `portainer_kubernetes_storage`             | [kubernetes_storage.md](docs/resources/kubernetes_storage.md)                                  | [example](examples/kubernetes_storage/)              | ✅     | ❌ / ❌                             | ✅        |
| `portainer_alerting_rule`                  | [alerting_rule.md](docs/resources/alerting_rule.md)                                            | [example](examples/alerting_rule/)                   | ✅     | ❌ / ✅                             | ❌        |
| `portainer_alerting_settings`              | [alerting_settings.md](docs/resources/alerting_settings.md)                                    | [example](examples/alerting_settings/)               | ✅     | ❌ / ✅                             | ❌        |
| `portainer_alerting_silence`               | [alerting_silence.md](docs/resources/alerting_silence.md)                                      | [example](examples/alerting_silence/)                | ✅     | ❌ / ❌                             | ❌        |
| `portainer_helm_user_repository`           | [helm_user_repository.md](docs/resources/helm_user_repository.md)                              | [example](examples/helm_user_repository/)            | ✅     | ✅ / ❌                             | ❌        |
| `portainer_policy`                         | [policy.md](docs/resources/policy.md)                                                          | [example](examples/policy/)                          | ✅     | ✅ / ✅                             | ❌        |
| `portainer_shared_git_credential`          | [shared_git_credential.md](docs/resources/shared_git_credential.md)                            | [example](examples/shared_git_credential/)           | ✅     | ✅ / ✅                             | ❌        |
| `portainer_stack_migrate`                  | [stack_migrate.md](docs/resources/stack_migrate.md)                                            | [example](examples/stack_migrate/)                   | ✅     | ❌ / ❌                             | ❌        |
| `portainer_user_git_credential`            | [user_git_credential.md](docs/resources/user_git_credential.md)                                | [example](examples/user_git_credential/)             | ✅     | ✅ / ✅                             | ❌        |
| `portainer_ldap_settings`                  | [ldap_settings.md](docs/resources/ldap_settings.md)                                            | [example](examples/ldap_settings/)                   | ✅     | ❌ / ✅                             | ❌        |
| `portainer_helm_rollback`                  | [helm_rollback.md](docs/resources/helm_rollback.md)                                            | [example](examples/helm_rollback/)                   | ✅     | ❌ / ❌                             | ❌        |
| `portainer_gitops_source`                  | [gitops_source.md](docs/resources/gitops_source.md)                                            | [example](examples/gitops_source/)                   | ✅     | ✅ / ✅                             | ❌        |
| `portainer_user_api_key`                   | [user_api_key.md](docs/resources/user_api_key.md)                                              | [example](examples/user_api_key/)                    | ✅     | ❌ / ❌                             | ❌        |
| `portainer_registry_configure`             | [registry_configure.md](docs/resources/registry_configure.md)                                  | [example](examples/registry_configure/)              | ✅     | ❌ / ❌                             | ❌        |
| `portainer_endpoint_relations`             | [endpoint_relations.md](docs/resources/endpoint_relations.md)                                  | [example](examples/endpoint_relations/)              | ✅     | ❌ / ❌                             | ❌        |
| `portainer_restore`                        | [restore.md](docs/resources/restore.md)                                                        | [example](examples/restore/)                         | ✅     | ❌ / ❌                             | ❌        |
| `portainer_stack_delete_by_name`           | [stack_delete_by_name.md](docs/resources/stack_delete_by_name.md)                              | [example](examples/stack_delete_by_name/)            | ✅     | ❌ / ❌                             | ❌        |
| `portainer_edge_job_task_logs`             | [edge_job_task_logs.md](docs/resources/edge_job_task_logs.md)                                  | [example](examples/edge_job_task_logs/)              | ✅     | ❌ / ❌                             | ❌        |
| `portainer_kubernetes_persistent_volume`   | [kubernetes_persistent_volume.md](docs/resources/kubernetes_persistent_volume.md)              | [example](examples/kubernetes_persistent_volume/)    | ✅     | ❌ / ✅                             | ❌        |
| `portainer_endpoint_group_membership`      | [endpoint_group_membership.md](docs/resources/endpoint_group_membership.md)                    | [example](examples/endpoint_group_membership/)       | ✅     | ❌ / ❌                             | ❌        |



## 📊 Supported Data Sources
| Data Source                   | Documentation                                                     | Example                                           | Status | E2E Tests |
|------------------------------|-------------------------------------------------------------------|---------------------------------------------------|--------|-----------|
| `portainer_user`              | [user.md](docs/data-sources/user.md)                             | [user resource](docs/resources/user.md)      | ✅     | ✅        |
| `portainer_team`              | [team.md](docs/data-sources/team.md)                             | [team resource](docs/resources/team.md)      | ✅     | ✅        |
| `portainer_environment`       | [environment.md](docs/data-sources/environment.md)               | [env resource](docs/resources/environment.md)| ✅     | ❌        |
| `portainer_endpoint_group`    | [endpoint_group.md](docs/data-sources/endpoint_group.md)         | [endpoint group docs](docs/data-sources/endpoint_group.md) | ✅     | ❌        |
| `portainer_tag`               | [tag.md](docs/data-sources/tag.md)                               | [tag resource](docs/resources/tag.md)        | ✅     | ❌        |
| `portainer_registry`          | [registry.md](docs/data-sources/registry.md)                     | [registry resource](docs/resources/registry.md) | ✅     | ✅        |
| `portainer_stack`             | [stack.md](docs/data-sources/stack.md)                           | [stack resource](docs/resources/stack.md)    | ✅     | ❌        |
| `portainer_edge_group`        | [edge_group.md](docs/data-sources/edge_group.md)                 | [edge group docs](docs/data-sources/edge_group.md)   | ✅     | ❌        |
| `portainer_custom_template`   | [custom_template.md](docs/data-sources/custom_template.md)       | [custom template docs](docs/data-sources/custom_template.md) | ✅     | ❌        |
| `portainer_cloud_credentials` | [cloud_credentials.md](docs/data-sources/cloud_credentials.md)   | [cloud credentials docs](docs/data-sources/cloud_credentials.md) | ✅     | ❌        |
| `portainer_edge_stack`        | [edge_stack.md](docs/data-sources/edge_stack.md)                 | [edge stack docs](docs/data-sources/edge_stack.md)   | ✅     | ❌        |
| `portainer_edge_job`          | [edge_job.md](docs/data-sources/edge_job.md)                     | [edge job docs](docs/data-sources/edge_job.md)     | ✅     | ❌        |
| `portainer_edge_configuration`| [edge_configuration.md](docs/data-sources/edge_configuration.md) | [edge configuration docs](docs/data-sources/edge_configuration.md) | ✅     | ❌        |
| `portainer_webhook`           | [webhook.md](docs/data-sources/webhook.md)                       | [webhook docs](docs/data-sources/webhook.md)      | ✅     | ❌        |
| `portainer_docker_network`    | [docker_network.md](docs/data-sources/docker_network.md)         | [docker network docs](docs/data-sources/docker_network.md) | ✅     | ✅        |
| `portainer_docker_volume`     | [docker_volume.md](docs/data-sources/docker_volume.md)           | [docker volume docs](docs/data-sources/docker_volume.md) | ✅     | ❌        |
| `portainer_docker_config`     | [docker_config.md](docs/data-sources/docker_config.md)           | [docker config docs](docs/data-sources/docker_config.md) | ✅     | ❌        |
| `portainer_docker_secret`     | [docker_secret.md](docs/data-sources/docker_secret.md)           | [docker secret docs](docs/data-sources/docker_secret.md) | ✅     | ❌        |
| `portainer_docker_image`      | [docker_image.md](docs/data-sources/docker_image.md)             | [docker image docs](docs/data-sources/docker_image.md) | ✅     | ❌        |
| `portainer_docker_node`       | [docker_node.md](docs/data-sources/docker_node.md)               | [docker node docs](docs/data-sources/docker_node.md) | ✅     | ❌        |
| `portainer_team_membership`   | [team_membership.md](docs/data-sources/team_membership.md)       | [team membership docs](docs/data-sources/team_membership.md)| ✅     | ❌        |
| `portainer_endpoint_group_access` | [endpoint_group_access.md](docs/data-sources/endpoint_group_access.md) | [endpoint group access docs](docs/data-sources/endpoint_group_access.md) | ✅ | ❌        |
| `portainer_registry_access`   | [registry_access.md](docs/data-sources/registry_access.md)       | [registry access docs](docs/data-sources/registry_access.md)| ✅     | ❌        |
| `portainer_kubernetes_crd`    | [kubernetes_crd.md](docs/data-sources/kubernetes_crd.md)         | [kubernetes crd docs](docs/data-sources/kubernetes_crd.md)   | ✅     | ❌        |
| `portainer_kubernetes_manifest_dry_run` | [kubernetes_manifest_dry_run.md](docs/data-sources/kubernetes_manifest_dry_run.md) | [example](examples/data_kubernetes_manifest_dry_run/) | ✅     | ✅        |
| `portainer_kubernetes_ingress_classes` | [kubernetes_ingress_classes.md](docs/data-sources/kubernetes_ingress_classes.md) | [example](examples/data_kubernetes_ingress_classes/) | ✅     | ✅        |
| `portainer_kubernetes_resource_quotas` | [kubernetes_resource_quotas.md](docs/data-sources/kubernetes_resource_quotas.md) | [example](examples/data_kubernetes_resource_quotas/) | ✅     | ✅        |
| `portainer_kubernetes_replicasets` | [kubernetes_replicasets.md](docs/data-sources/kubernetes_replicasets.md) | [example](examples/data_kubernetes_replicasets/) | ✅     | ✅        |
| `portainer_kubernetes_deployments` | [kubernetes_deployments.md](docs/data-sources/kubernetes_deployments.md) | [example](examples/data_kubernetes_deployments/) | ✅     | ✅        |
| `portainer_kubernetes_pods` | [kubernetes_pods.md](docs/data-sources/kubernetes_pods.md) | [example](examples/data_kubernetes_pods/) | ✅     | ✅        |
| `portainer_kubernetes_pod_logs` | [kubernetes_pod_logs.md](docs/data-sources/kubernetes_pod_logs.md) | [example](examples/data_kubernetes_pod_logs/) | ✅     | ❌        |
| `portainer_policy`            | [policy.md](docs/data-sources/policy.md)                         | [policy docs](docs/data-sources/policy.md)                   | ✅     | ❌        |
| `portainer_policy_template`   | [policy_template.md](docs/data-sources/policy_template.md)       | [policy template docs](docs/data-sources/policy_template.md) | ✅     | ❌        |
| `portainer_role`              | [role.md](docs/data-sources/role.md)                             | [role docs](docs/data-sources/role.md)                       | ✅     | ❌        |
| `portainer_shared_git_credential` | [shared_git_credential.md](docs/data-sources/shared_git_credential.md) | [shared git credential docs](docs/data-sources/shared_git_credential.md) | ✅ | ❌        |
| `portainer_user_activity`     | [user_activity.md](docs/data-sources/user_activity.md)           | [user activity docs](docs/data-sources/user_activity.md)     | ✅     | ❌        |
| `portainer_helm_git_dryrun`   | [helm_git_dryrun.md](docs/data-sources/helm_git_dryrun.md)      | [helm git dryrun docs](docs/data-sources/helm_git_dryrun.md) | ✅     | ❌        |
| `portainer_gitops_repo_refs`  | [gitops_repo_refs.md](docs/data-sources/gitops_repo_refs.md)    | [gitops repo refs docs](docs/data-sources/gitops_repo_refs.md) | ✅   | ❌        |
| `portainer_gitops_repo_file`  | [gitops_repo_file.md](docs/data-sources/gitops_repo_file.md)    | [gitops repo file docs](docs/data-sources/gitops_repo_file.md) | ✅   | ❌        |
| `portainer_gitops_source` | [gitops_source.md](docs/data-sources/gitops_source.md) | [example](examples/data_gitops_source/) | ✅     | ❌        |
| `portainer_gitops_sources` | [gitops_sources.md](docs/data-sources/gitops_sources.md) | [example](examples/data_gitops_sources/) | ✅     | ❌        |
| `portainer_gitops_workflow` | [gitops_workflow.md](docs/data-sources/gitops_workflow.md) | [example](examples/data_gitops_workflow/) | ✅     | ❌        |
| `portainer_gitops_workflows` | [gitops_workflows.md](docs/data-sources/gitops_workflows.md) | [example](examples/data_gitops_workflows/) | ✅     | ❌        |
| `portainer_gitops_source_connection` | [gitops_source_connection.md](docs/data-sources/gitops_source_connection.md) | [example](examples/data_gitops_source_connection/) | ✅     | ❌        |
| `portainer_registry_connection` | [registry_connection.md](docs/data-sources/registry_connection.md) | [example](examples/data_registry_connection/) | ✅     | ❌        |
| `portainer_ldap_check` | [ldap_check.md](docs/data-sources/ldap_check.md) | [example](examples/data_ldap_check/) | ✅     | ❌        |
| `portainer_backup_azure_connection` | [backup_azure_connection.md](docs/data-sources/backup_azure_connection.md) | [example](examples/data_backup_azure_connection/) | ✅     | ❌        |
| `portainer_addons` | [addons.md](docs/data-sources/addons.md) | [example](examples/data_addons/) | ✅     | ❌        |
| `portainer_addon_chart_source` | [addon_chart_source.md](docs/data-sources/addon_chart_source.md) | [example](examples/data_addon_chart_source/) | ✅     | ❌        |
| `portainer_edge_mtls_ca_certificate` | [edge_mtls_ca_certificate.md](docs/data-sources/edge_mtls_ca_certificate.md) | [example](examples/data_edge_mtls_ca_certificate/) | ✅     | ❌        |
| `portainer_edge_mtls_certificate` | [edge_mtls_certificate.md](docs/data-sources/edge_mtls_certificate.md) | [example](examples/data_edge_mtls_certificate/) | ✅     | ❌        |
| `portainer_endpoint_mtls_certificate` | [endpoint_mtls_certificate.md](docs/data-sources/endpoint_mtls_certificate.md) | [example](examples/data_endpoint_mtls_certificate/) | ✅     | ❌        |
| `portainer_endpoint_mtls_certificate_error` | [endpoint_mtls_certificate_error.md](docs/data-sources/endpoint_mtls_certificate_error.md) | [example](examples/data_endpoint_mtls_certificate_error/) | ✅     | ❌        |
| `portainer_edge_waiting_room` | [edge_waiting_room.md](docs/data-sources/edge_waiting_room.md) | [example](examples/data_edge_waiting_room/) | ✅     | ❌        |
| `portainer_alerting_rule_environments` | [alerting_rule_environments.md](docs/data-sources/alerting_rule_environments.md) | [example](examples/data_alerting_rule_environments/) | ✅     | ❌        |
| `portainer_auto_updates` | [auto_updates.md](docs/data-sources/auto_updates.md) | [example](examples/data_auto_updates/) | ✅     | ❌        |
| `portainer_omni_machines` | [omni_machines.md](docs/data-sources/omni_machines.md) | [example](examples/data_omni_machines/) | ✅     | ❌        |
| `portainer_omni_machine` | [omni_machine.md](docs/data-sources/omni_machine.md) | [example](examples/data_omni_machine/) | ✅     | ❌        |
| `portainer_omni_machine_logs` | [omni_machine_logs.md](docs/data-sources/omni_machine_logs.md) | [example](examples/data_omni_machine_logs/) | ✅     | ❌        |
| `portainer_omni_talos_versions` | [omni_talos_versions.md](docs/data-sources/omni_talos_versions.md) | [example](examples/data_omni_talos_versions/) | ✅     | ❌        |
| `portainer_omni_upgrade_status` | [omni_upgrade_status.md](docs/data-sources/omni_upgrade_status.md) | [example](examples/data_omni_upgrade_status/) | ✅     | ❌        |
| `portainer_omni_service_account` | [omni_service_account.md](docs/data-sources/omni_service_account.md) | [example](examples/data_omni_service_account/) | ✅     | ❌        |
| `portainer_kubernetes_custom_resource_definitions` | [kubernetes_custom_resource_definitions.md](docs/data-sources/kubernetes_custom_resource_definitions.md) | [example](examples/data_kubernetes_custom_resource_definitions/) | ✅     | ❌        |
| `portainer_kubernetes_custom_resource_definition` | [kubernetes_custom_resource_definition.md](docs/data-sources/kubernetes_custom_resource_definition.md) | [example](examples/data_kubernetes_custom_resource_definition/) | ✅     | ❌        |
| `portainer_kubernetes_custom_resources` | [kubernetes_custom_resources.md](docs/data-sources/kubernetes_custom_resources.md) | [example](examples/data_kubernetes_custom_resources/) | ✅     | ❌        |
| `portainer_kubernetes_custom_resource` | [kubernetes_custom_resource.md](docs/data-sources/kubernetes_custom_resource.md) | [example](examples/data_kubernetes_custom_resource/) | ✅     | ❌        |
| `portainer_kubernetes_gpu` | [kubernetes_gpu.md](docs/data-sources/kubernetes_gpu.md) | [example](examples/data_kubernetes_gpu/) | ✅     | ❌        |
| `portainer_stack_conversion` | [stack_conversion.md](docs/data-sources/stack_conversion.md) | [example](examples/data_stack_conversion/) | ✅     | ❌        |
| `portainer_kubernetes_storage_classes` | [kubernetes_storage_classes.md](docs/data-sources/kubernetes_storage_classes.md) | [example](examples/data_kubernetes_storage_classes/) | ✅     | ❌        |
| `portainer_kubernetes_storage_class` | [kubernetes_storage_class.md](docs/data-sources/kubernetes_storage_class.md) | [example](examples/data_kubernetes_storage_class/) | ✅     | ❌        |
| `portainer_kubernetes_persistent_volume_claims` | [kubernetes_persistent_volume_claims.md](docs/data-sources/kubernetes_persistent_volume_claims.md) | [example](examples/data_kubernetes_persistent_volume_claims/) | ✅     | ❌        |
| `portainer_kubernetes_persistent_volume_claim` | [kubernetes_persistent_volume_claim.md](docs/data-sources/kubernetes_persistent_volume_claim.md) | [example](examples/data_kubernetes_persistent_volume_claim/) | ✅     | ❌        |
| `portainer_kubernetes_volumes` | [kubernetes_volumes.md](docs/data-sources/kubernetes_volumes.md) | [example](examples/data_kubernetes_volumes/) | ✅     | ❌        |
| `portainer_kubernetes_volume` | [kubernetes_volume.md](docs/data-sources/kubernetes_volume.md) | [example](examples/data_kubernetes_volume/) | ✅     | ❌        |
| `portainer_kubernetes_cron_jobs` | [kubernetes_cron_jobs.md](docs/data-sources/kubernetes_cron_jobs.md) | [example](examples/data_kubernetes_cron_jobs/) | ✅     | ❌        |
| `portainer_kubernetes_endpoints` | [kubernetes_endpoints.md](docs/data-sources/kubernetes_endpoints.md) | [example](examples/data_kubernetes_endpoints/) | ✅     | ❌        |
| `portainer_kubernetes_service_account` | [kubernetes_service_account.md](docs/data-sources/kubernetes_service_account.md) | [example](examples/data_kubernetes_service_account/) | ✅     | ❌        |
| `portainer_kubernetes_application` | [kubernetes_application.md](docs/data-sources/kubernetes_application.md) | [example](examples/data_kubernetes_application/) | ✅     | ❌        |
| `portainer_kubernetes_resource_counts` | [kubernetes_resource_counts.md](docs/data-sources/kubernetes_resource_counts.md) | [example](examples/data_kubernetes_resource_counts/) | ✅     | ❌        |
| `portainer_ldap_users` | [ldap_users.md](docs/data-sources/ldap_users.md) | [example](examples/data_ldap_users/) | ✅     | ❌        |
| `portainer_ldap_groups` | [ldap_groups.md](docs/data-sources/ldap_groups.md) | [example](examples/data_ldap_groups/) | ✅     | ❌        |
| `portainer_ldap_admin_groups` | [ldap_admin_groups.md](docs/data-sources/ldap_admin_groups.md) | [example](examples/data_ldap_admin_groups/) | ✅     | ❌        |
| `portainer_ldap_login_test` | [ldap_login_test.md](docs/data-sources/ldap_login_test.md) | [example](examples/data_ldap_login_test/) | ✅     | ❌        |
| `portainer_user_namespaces` | [user_namespaces.md](docs/data-sources/user_namespaces.md) | [example](examples/data_user_namespaces/) | ✅     | ❌        |
| `portainer_current_user_authorizations` | [current_user_authorizations.md](docs/data-sources/current_user_authorizations.md) | [example](examples/data_current_user_authorizations/) | ✅     | ❌        |
| `portainer_licenses_info` | [licenses_info.md](docs/data-sources/licenses_info.md) | [example](examples/data_licenses_info/) | ✅     | ❌        |
| `portainer_recommendations` | [recommendations.md](docs/data-sources/recommendations.md) | [example](examples/data_recommendations/) | ✅     | ❌        |
| `portainer_policy_metadata` | [policy_metadata.md](docs/data-sources/policy_metadata.md) | [example](examples/data_policy_metadata/) | ✅     | ❌        |
| `portainer_policy_conflicts` | [policy_conflicts.md](docs/data-sources/policy_conflicts.md) | [example](examples/data_policy_conflicts/) | ✅     | ❌        |
| `portainer_policy_observability_test` | [policy_observability_test.md](docs/data-sources/policy_observability_test.md) | [example](examples/data_policy_observability_test/) | ✅     | ❌        |
| `portainer_alerting_connectivity` | [alerting_connectivity.md](docs/data-sources/alerting_connectivity.md) | [example](examples/data_alerting_connectivity/) | ✅     | ❌        |
| `portainer_environment_logs` | [environment_logs.md](docs/data-sources/environment_logs.md) | [example](examples/data_environment_logs/) | ✅     | ❌        |
| `portainer_environment_metrics` | [environment_metrics.md](docs/data-sources/environment_metrics.md) | [example](examples/data_environment_metrics/) | ✅     | ❌        |
| `portainer_docker_snapshot` | [docker_snapshot.md](docs/data-sources/docker_snapshot.md) | [example](examples/data_docker_snapshot/) | ✅     | ❌        |
| `portainer_docker_snapshot_containers` | [docker_snapshot_containers.md](docs/data-sources/docker_snapshot_containers.md) | [example](examples/data_docker_snapshot_containers/) | ✅     | ❌        |
| `portainer_docker_snapshot_container` | [docker_snapshot_container.md](docs/data-sources/docker_snapshot_container.md) | [example](examples/data_docker_snapshot_container/) | ✅     | ❌        |
| `portainer_image_status` | [image_status.md](docs/data-sources/image_status.md) | [example](examples/data_image_status/) | ✅     | ❌        |
| `portainer_edge_update_schedule_info` | [edge_update_schedule_info.md](docs/data-sources/edge_update_schedule_info.md) | [example](examples/data_edge_update_schedule_info/) | ✅     | ❌        |
| `portainer_agent_versions` | [agent_versions.md](docs/data-sources/agent_versions.md) | [example](examples/data_agent_versions/) | ✅     | ❌        |
| `portainer_edge_update_previous_versions` | [edge_update_previous_versions.md](docs/data-sources/edge_update_previous_versions.md) | [example](examples/data_edge_update_previous_versions/) | ✅     | ❌        |
| `portainer_edge_update_schedules_active` | [edge_update_schedules_active.md](docs/data-sources/edge_update_schedules_active.md) | [example](examples/data_edge_update_schedules_active/) | ✅     | ❌        |
| `portainer_edge_configuration_files` | [edge_configuration_files.md](docs/data-sources/edge_configuration_files.md) | [example](examples/data_edge_configuration_files/) | ✅     | ❌        |
| `portainer_edge_stack_stagger_status` | [edge_stack_stagger_status.md](docs/data-sources/edge_stack_stagger_status.md) | [example](examples/data_edge_stack_stagger_status/) | ✅     | ❌        |
| `portainer_gitops_repo_file_search` | [gitops_repo_file_search.md](docs/data-sources/gitops_repo_file_search.md) | [example](examples/data_gitops_repo_file_search/) | ✅     | ❌        |
| `portainer_gitops_helm_values` | [gitops_helm_values.md](docs/data-sources/gitops_helm_values.md) | [example](examples/data_gitops_helm_values/) | ✅     | ❌        |
| `portainer_system` | [system.md](docs/data-sources/system.md) | [example](examples/data_system/) | ✅     | ❌        |
| `portainer_settings_public` | [settings_public.md](docs/data-sources/settings_public.md) | [example](examples/data_settings_public/) | ✅     | ❌        |
| `portainer_motd` | [motd.md](docs/data-sources/motd.md) | [example](examples/data_motd/) | ✅     | ❌        |
| `portainer_user_access` | [user_access.md](docs/data-sources/user_access.md) | [example](examples/data_user_access/) | ✅     | ❌        |
| `portainer_kubernetes_cluster` | [kubernetes_cluster.md](docs/data-sources/kubernetes_cluster.md) | [example](examples/data_kubernetes_cluster/) | ✅     | ❌        |
| `portainer_kubernetes_nodes` | [kubernetes_nodes.md](docs/data-sources/kubernetes_nodes.md) | [example](examples/data_kubernetes_nodes/) | ✅     | ❌        |
| `portainer_kubernetes_events` | [kubernetes_events.md](docs/data-sources/kubernetes_events.md) | [example](examples/data_kubernetes_events/) | ✅     | ❌        |
| `portainer_kubernetes_describe` | [kubernetes_describe.md](docs/data-sources/kubernetes_describe.md) | [example](examples/data_kubernetes_describe/) | ✅     | ❌        |
| `portainer_kubernetes_persistent_volumes` | [kubernetes_persistent_volumes.md](docs/data-sources/kubernetes_persistent_volumes.md) | [example](examples/data_kubernetes_persistent_volumes/) | ✅     | ❌        |
| `portainer_kubernetes_config` | [kubernetes_config.md](docs/data-sources/kubernetes_config.md) | [example](examples/data_kubernetes_config/) | ✅     | ❌        |
| `portainer_docker_dashboard` | [docker_dashboard.md](docs/data-sources/docker_dashboard.md) | [example](examples/data_docker_dashboard/) | ✅     | ❌        |
| `portainer_docker_images` | [docker_images.md](docs/data-sources/docker_images.md) | [example](examples/data_docker_images/) | ✅     | ❌        |
| `portainer_docker_container_gpus` | [docker_container_gpus.md](docs/data-sources/docker_container_gpus.md) | [example](examples/data_docker_container_gpus/) | ✅     | ❌        |
| `portainer_app_templates` | [app_templates.md](docs/data-sources/app_templates.md) | [example](examples/data_app_templates/) | ✅     | ❌        |
| `portainer_helm_chart` | [helm_chart.md](docs/data-sources/helm_chart.md) | [example](examples/data_helm_chart/) | ✅     | ❌        |
| `portainer_custom_template_file` | [custom_template_file.md](docs/data-sources/custom_template_file.md) | [example](examples/data_custom_template_file/) | ✅     | ❌        |
| `portainer_edge_stack_file` | [edge_stack_file.md](docs/data-sources/edge_stack_file.md) | [example](examples/data_edge_stack_file/) | ✅     | ❌        |
| `portainer_edge_job_file` | [edge_job_file.md](docs/data-sources/edge_job_file.md) | [example](examples/data_edge_job_file/) | ✅     | ❌        |
| `portainer_edge_job_tasks` | [edge_job_tasks.md](docs/data-sources/edge_job_tasks.md) | [example](examples/data_edge_job_tasks/) | ✅     | ❌        |
| `portainer_edge_job_task_logs` | [edge_job_task_logs.md](docs/data-sources/edge_job_task_logs.md) | [example](examples/data_edge_job_task_logs/) | ✅     | ❌        |
| `portainer_endpoints_summary` | [endpoints_summary.md](docs/data-sources/endpoints_summary.md) | [example](examples/data_endpoints_summary/) | ✅     | ❌        |
| `portainer_endpoint_registries` | [endpoint_registries.md](docs/data-sources/endpoint_registries.md) | [example](examples/data_endpoint_registries/) | ✅     | ❌        |
| `portainer_team_memberships` | [team_memberships.md](docs/data-sources/team_memberships.md) | [example](examples/data_team_memberships/) | ✅     | ❌        |
| `portainer_kubernetes_pod_metrics` | [kubernetes_pod_metrics.md](docs/data-sources/kubernetes_pod_metrics.md) | [example](examples/data_kubernetes_pod_metrics/) | ✅     | ❌        |
| `portainer_kubernetes_node_metrics` | [kubernetes_node_metrics.md](docs/data-sources/kubernetes_node_metrics.md) | [example](examples/data_kubernetes_node_metrics/) | ✅     | ❌        |
| `portainer_kubernetes_application_resources` | [kubernetes_application_resources.md](docs/data-sources/kubernetes_application_resources.md) | [example](examples/data_kubernetes_application_resources/) | ✅     | ❌        |
| `portainer_helm_release_history` | [helm_release_history.md](docs/data-sources/helm_release_history.md) | [helm release history docs](docs/data-sources/helm_release_history.md) | ✅ | ❌ |


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
