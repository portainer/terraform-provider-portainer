# Portainer Provider

A Terraform provider to manage [Portainer](https://www.portainer.io/) resources via its REST API using Terraform.

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

  # Optional: add custom headers to ALL requests (e.g. Cloudflare Access / auth proxy)
  # custom_headers = {
  #   "CF-Access-Client-Id"     = "..."
  #   "CF-Access-Client-Secret" = "..."
  # }

  # Optional: retries for transient failures and a per-request time limit
  # max_retries     = 3    # default
  # request_timeout = "5m" # default: no limit
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

### HTTP Proxy Support
The provider honors the standard `HTTP_PROXY`, `HTTPS_PROXY` and `NO_PROXY` environment variables (same behavior as Go's `http.DefaultTransport`). This lets you reach a Portainer endpoint that is only accessible through an HTTP(S) proxy – no provider configuration is required.

```hcl
$ export HTTPS_PROXY="http://proxy.example.com:8080"
$ export NO_PROXY="localhost,127.0.0.1,.internal.example.com"
```

> These variables are a no-op when unset, so existing setups are unaffected. Lowercase variants (`http_proxy`, `https_proxy`, `no_proxy`) are also recognized.

### Retries and Timeouts
Requests that fail transiently are retried up to `max_retries` times (default 3) with exponential backoff, honoring a `Retry-After` header. Only retries that cannot apply a change twice are made:

| Failure | Retried for |
| ------- | ----------- |
| Connection refused (the request never reached Portainer) | every method |
| `429 Too Many Requests`, `503 Service Unavailable` | `GET`, `PUT`, `DELETE` |
| `502 Bad Gateway`, `504 Gateway Timeout`, dropped connection | `GET` only, because a proxy can answer this while Portainer still applies the change |

`request_timeout` limits a single request, including reading the response. It is unset by default: pulling a large image or installing a Helm chart is one request that can legitimately take minutes. Every request is still cancelled when the Terraform operation times out or is interrupted.

```hcl
provider "portainer" {
  endpoint        = "https://portainer.example.com"
  max_retries     = 5
  request_timeout = "5m"
}
```

## Arguments Reference
| Name              | Type    | Required | Description                                                                                        |
| ----------------- | ------- | -------- | ---------------------------------------------------------------------------------------------------|
| `endpoint`        | string  | ✅ yes   | URL of the Portainer instance. `/api` will be appended automatically if missing.                   |
| `api_key`         | string  | ❌ no    | API key for authentication. Mutually exclusive with `api_user` and `api_password`.                 |
| `api_user`        | string  | ❌ no    | Username for authentication (must be used with `api_password`). Mutually exclusive with `api_key`. |
| `api_password`    | string  | ❌ no    | Password for authentication (must be used with `api_user`). Mutually exclusive with `api_key`.     |
| `skip_ssl_verify` | boolean | ❌ no    | Skip TLS certificate verification (useful for self-signed certs). Default: `false`.                |
| `custom_headers`  | map(string) | ❌ no | Custom headers added to all requests (e.g. Cloudflare Access / security proxy headers).            |
| `max_retries`     | number  | ❌ no    | Retries for transient failures, with exponential backoff (1s, 2s, 4s, ...). `0` disables. Default: `3`. Env: `PORTAINER_MAX_RETRIES`. |
| `request_timeout` | string  | ❌ no    | Time limit for one HTTP request, as a duration such as `"2m"`. Default: no limit. Env: `PORTAINER_REQUEST_TIMEOUT`. |

## 🧩 Supported Resources
| Resource                                       | Status                                                                |
|------------------------------------------------|-----------------------------------------------------------------------|
| `portainer_auth`                               | ![Done](https://img.shields.io/badge/status-done-brightgreen)         |
| `portainer_user_admin`                         | ![Done](https://img.shields.io/badge/status-done-brightgreen)         |
| `portainer_backup`                             | ![Done](https://img.shields.io/badge/status-done-brightgreen)         |
| `portainer_backup_s3`                          | ![Done](https://img.shields.io/badge/status-done-brightgreen)         |
| `portainer_backup_azure_settings`                | ![Done](https://img.shields.io/badge/status-done-brightgreen)         |
| `portainer_backup_azure_execute`                 | ![Done](https://img.shields.io/badge/status-done-brightgreen)         |
| `portainer_backup_azure_restore`                 | ![Done](https://img.shields.io/badge/status-done-brightgreen)         |
| `portainer_backup_local_settings`                | ![Done](https://img.shields.io/badge/status-done-brightgreen)         |
| `portainer_backup_local_run`                     | ![Done](https://img.shields.io/badge/status-done-brightgreen)         |
| `portainer_backup_s3_restore`                    | ![Done](https://img.shields.io/badge/status-done-brightgreen)         |
| `portainer_addon` | ![Done](https://img.shields.io/badge/status-done-brightgreen)         |
| `portainer_addon_access` | ![Done](https://img.shields.io/badge/status-done-brightgreen)         |
| `portainer_addon_config` | ![Done](https://img.shields.io/badge/status-done-brightgreen)         |
| `portainer_addon_repair` | ![Done](https://img.shields.io/badge/status-done-brightgreen)         |
| `portainer_addon_catalog_refresh` | ![Done](https://img.shields.io/badge/status-done-brightgreen)         |
| `portainer_settings_default_registry` | ![Done](https://img.shields.io/badge/status-done-brightgreen)         |
| `portainer_settings_additional_functionality` | ![Done](https://img.shields.io/badge/status-done-brightgreen)         |
| `portainer_endpoint_trust` | ![Done](https://img.shields.io/badge/status-done-brightgreen)         |
| `portainer_alerting_rule_groups` | ![Done](https://img.shields.io/badge/status-done-brightgreen)         |
| `portainer_alerting_rule_tiers` | ![Done](https://img.shields.io/badge/status-done-brightgreen)         |
| `portainer_ssrf_allowlist` | ![Done](https://img.shields.io/badge/status-done-brightgreen)         |
| `portainer_omni_cluster` | ![Done](https://img.shields.io/badge/status-done-brightgreen)         |
| `portainer_omni_node_reboot` | ![Done](https://img.shields.io/badge/status-done-brightgreen)         |
| `portainer_kubernetes_pod_security_rule` | ![Done](https://img.shields.io/badge/status-done-brightgreen)         |
| `portainer_gitops_workflow` | ![Done](https://img.shields.io/badge/status-done-brightgreen)         |
| `portainer_gitops_workflow_reconcile` | ![Done](https://img.shields.io/badge/status-done-brightgreen)         |
| `portainer_kubernetes_cluster_upgrade` | ![Done](https://img.shields.io/badge/status-done-brightgreen)         |
| `portainer_user_memberships_sync` | ![Done](https://img.shields.io/badge/status-done-brightgreen)         |
| `portainer_chat`                               | ![Done](https://img.shields.io/badge/status-done-brightgreen)         |
| `portainer_cloud_credentials`                  | ![Done](https://img.shields.io/badge/status-done-brightgreen)         |
| `portainer_cloud_provider_provision`           | ![Done](https://img.shields.io/badge/status-done-brightgreen)         |
| `portainer_deploy`                             | ![Done](https://img.shields.io/badge/status-done-brightgreen)         |
| `portainer_check`                              | ![Done](https://img.shields.io/badge/status-done-brightgreen)         |
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
| `portainer_endpoint_group_access`              | ![Done](https://img.shields.io/badge/status-done-brightgreen)         |
| `portainer_endpoint_service_update`            | ![Done](https://img.shields.io/badge/status-done-brightgreen)         |
| `portainer_endpoint_settings`                  | ![Done](https://img.shields.io/badge/status-done-brightgreen)         |
| `portainer_endpoint_snapshot`                  | ![Done](https://img.shields.io/badge/status-done-brightgreen)         |
| `portainer_endpoints_edge_generate_key`        | ![Done](https://img.shields.io/badge/status-done-brightgreen)         |
| `portainer_environment`                        | ![Done](https://img.shields.io/badge/status-done-brightgreen)         |
| `portainer_d2k_environment`                    | ![Done](https://img.shields.io/badge/status-done-brightgreen)         |
| `portainer_kubesolo_offline_bundle`            | ![Done](https://img.shields.io/badge/status-done-brightgreen)         |
| `portainer_kubernetes_application`             | ![Done](https://img.shields.io/badge/status-done-brightgreen)         |
| `portainer_kubernetes_clusterrole`             | ![Done](https://img.shields.io/badge/status-done-brightgreen)         |
| `portainer_kubernetes_clusterrolebinding`      | ![Done](https://img.shields.io/badge/status-done-brightgreen)         |
| `portainer_kubernetes_configmaps`              | ![Done](https://img.shields.io/badge/status-done-brightgreen)         |
| `portainer_kubernetes_cronjob`                 | ![Done](https://img.shields.io/badge/status-done-brightgreen)         |
| `portainer_kubernetes_delete_object`           | ![Done](https://img.shields.io/badge/status-done-brightgreen)         |
| `portainer_kubernetes_helm`                    | ![Done](https://img.shields.io/badge/status-done-brightgreen)         |
| `portainer_kubernetes_ingress`                 | ![Done](https://img.shields.io/badge/status-done-brightgreen)         |
| `portainer_kubernetes_gateway`                 | ![Done](https://img.shields.io/badge/status-done-brightgreen)         |
| `portainer_kubernetes_gateway_http_route`      | ![Done](https://img.shields.io/badge/status-done-brightgreen)         |
| `portainer_kubernetes_scoped_kubeconfig`       | ![Done](https://img.shields.io/badge/status-done-brightgreen)         |
| `portainer_kubernetes_ingresscontrollers`      | ![Done](https://img.shields.io/badge/status-done-brightgreen)         |
| `portainer_kubernetes_job`                     | ![Done](https://img.shields.io/badge/status-done-brightgreen)         |
| `portainer_kubernetes_namespace`               | ![Done](https://img.shields.io/badge/status-done-brightgreen)         |
| `portainer_kubernetes_namespace_access`        | ![Done](https://img.shields.io/badge/status-done-brightgreen)         |
| `portainer_kubernetes_namespace_ingresscontrollers` | ![Done](https://img.shields.io/badge/status-done-brightgreen)    |
| `portainer_kubernetes_namespace_system`        | ![Done](https://img.shields.io/badge/status-done-brightgreen)         |
| `portainer_kubernetes_role`                    | ![Done](https://img.shields.io/badge/status-done-brightgreen)         |
| `portainer_kubernetes_rolebinding`             | ![Done](https://img.shields.io/badge/status-done-brightgreen)         |
| `portainer_kubernetes_secret`                  | ![Done](https://img.shields.io/badge/status-done-brightgreen)         |
| `portainer_kubernetes_service`                 | ![Done](https://img.shields.io/badge/status-done-brightgreen)         |
| `portainer_kubernetes_service_account`         | ![Done](https://img.shields.io/badge/status-done-brightgreen)         |
| `portainer_kubernetes_storage`                 | ![Done](https://img.shields.io/badge/status-done-brightgreen)         |
| `portainer_kubernetes_volume`                  | ![Done](https://img.shields.io/badge/status-done-brightgreen)         |
| `portainer_kubernetes_node_drain`                          | ![Done](https://img.shields.io/badge/status-done-brightgreen)         |
| `portainer_kubernetes_deployment_scale`                    | ![Done](https://img.shields.io/badge/status-done-brightgreen)         |
| `portainer_kubernetes_deployment_rollback`                 | ![Done](https://img.shields.io/badge/status-done-brightgreen)         |
| `portainer_licenses`                           | ![Done](https://img.shields.io/badge/status-done-brightgreen)         |
| `portainer_registry`                           | ![Done](https://img.shields.io/badge/status-done-brightgreen)         |
| `portainer_registry_access`                    | ![Done](https://img.shields.io/badge/status-done-brightgreen)         |
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
| `portainer_kubernetes_namespace_access`        | ![Done](https://img.shields.io/badge/status-done-brightgreen)         |
| `portainer_alerting_rule`                      | ![Done](https://img.shields.io/badge/status-done-brightgreen)         |
| `portainer_alerting_settings`                  | ![Done](https://img.shields.io/badge/status-done-brightgreen)         |
| `portainer_alerting_silence`                   | ![Done](https://img.shields.io/badge/status-done-brightgreen)         |
| `portainer_helm_user_repository`               | ![Done](https://img.shields.io/badge/status-done-brightgreen)         |
| `portainer_policy`                             | ![Done](https://img.shields.io/badge/status-done-brightgreen)         |
| `portainer_shared_git_credential`              | ![Done](https://img.shields.io/badge/status-done-brightgreen)         |
| `portainer_stack_migrate`                      | ![Done](https://img.shields.io/badge/status-done-brightgreen)         |
| `portainer_user_git_credential`                | ![Done](https://img.shields.io/badge/status-done-brightgreen)         |
| `portainer_ldap_settings`                      | ![Done](https://img.shields.io/badge/status-done-brightgreen)         |
| `portainer_helm_rollback`                      | ![Done](https://img.shields.io/badge/status-done-brightgreen)         |
| `portainer_gitops_source`                      | ![Done](https://img.shields.io/badge/status-done-brightgreen)         |
| `portainer_user_api_key`                       | ![Done](https://img.shields.io/badge/status-done-brightgreen)         |
| `portainer_registry_configure`                 | ![Done](https://img.shields.io/badge/status-done-brightgreen)         |
| `portainer_endpoint_relations`                 | ![Done](https://img.shields.io/badge/status-done-brightgreen)         |
| `portainer_restore`                            | ![Done](https://img.shields.io/badge/status-done-brightgreen)         |
| `portainer_stack_delete_by_name`               | ![Done](https://img.shields.io/badge/status-done-brightgreen)         |
| `portainer_edge_job_task_logs`                 | ![Done](https://img.shields.io/badge/status-done-brightgreen)         |
| `portainer_kubernetes_persistent_volume`       | ![Done](https://img.shields.io/badge/status-done-brightgreen)         |
| `portainer_endpoint_group_membership`          | ![Done](https://img.shields.io/badge/status-done-brightgreen)         |

## 📊 Supported Data Sources
| Data Source                   | Status                                                        |
|------------------------------|---------------------------------------------------------------|
| `portainer_user`              | ![Done](https://img.shields.io/badge/status-done-brightgreen) |
| `portainer_team`              | ![Done](https://img.shields.io/badge/status-done-brightgreen) |
| `portainer_environment`       | ![Done](https://img.shields.io/badge/status-done-brightgreen) |
| `portainer_kubesolo_versions` | ![Done](https://img.shields.io/badge/status-done-brightgreen) |
| `portainer_kubesolo_offline_bundles` | ![Done](https://img.shields.io/badge/status-done-brightgreen) |
| `portainer_kubesolo_config`   | ![Done](https://img.shields.io/badge/status-done-brightgreen) |
| `portainer_endpoint_group`    | ![Done](https://img.shields.io/badge/status-done-brightgreen) |
| `portainer_tag`               | ![Done](https://img.shields.io/badge/status-done-brightgreen) |
| `portainer_registry`          | ![Done](https://img.shields.io/badge/status-done-brightgreen) |
| `portainer_stack`             | ![Done](https://img.shields.io/badge/status-done-brightgreen) |
| `portainer_edge_group`        | ![Done](https://img.shields.io/badge/status-done-brightgreen) |
| `portainer_custom_template`   | ![Done](https://img.shields.io/badge/status-done-brightgreen) |
| `portainer_cloud_credentials` | ![Done](https://img.shields.io/badge/status-done-brightgreen) |
| `portainer_edge_stack`        | ![Done](https://img.shields.io/badge/status-done-brightgreen) |
| `portainer_edge_job`          | ![Done](https://img.shields.io/badge/status-done-brightgreen) |
| `portainer_edge_configuration`| ![Done](https://img.shields.io/badge/status-done-brightgreen) |
| `portainer_webhook`           | ![Done](https://img.shields.io/badge/status-done-brightgreen) |
| `portainer_docker_network`    | ![Done](https://img.shields.io/badge/status-done-brightgreen) |
| `portainer_docker_volume`     | ![Done](https://img.shields.io/badge/status-done-brightgreen) |
| `portainer_docker_config`     | ![Done](https://img.shields.io/badge/status-done-brightgreen) |
| `portainer_docker_secret`     | ![Done](https://img.shields.io/badge/status-done-brightgreen) |
| `portainer_docker_image`      | ![Done](https://img.shields.io/badge/status-done-brightgreen) |
| `portainer_docker_node`       | ![Done](https://img.shields.io/badge/status-done-brightgreen) |
| `portainer_kubernetes_crd`    | ![Done](https://img.shields.io/badge/status-done-brightgreen) |
| `portainer_kubernetes_manifest_dry_run`   | ![Done](https://img.shields.io/badge/status-done-brightgreen) |
| `portainer_kubernetes_ingress_classes`    | ![Done](https://img.shields.io/badge/status-done-brightgreen) |
| `portainer_kubernetes_gateway_classes`    | ![Done](https://img.shields.io/badge/status-done-brightgreen) |
| `portainer_kubernetes_scoped_kubeconfigs` | ![Done](https://img.shields.io/badge/status-done-brightgreen) |
| `portainer_kubernetes_namespace_access`   | ![Done](https://img.shields.io/badge/status-done-brightgreen) |
| `portainer_kubernetes_resource_quotas`    | ![Done](https://img.shields.io/badge/status-done-brightgreen) |
| `portainer_kubernetes_replicasets`        | ![Done](https://img.shields.io/badge/status-done-brightgreen) |
| `portainer_kubernetes_deployments`        | ![Done](https://img.shields.io/badge/status-done-brightgreen) |
| `portainer_kubernetes_pods`               | ![Done](https://img.shields.io/badge/status-done-brightgreen) |
| `portainer_kubernetes_pod_logs`           | ![Done](https://img.shields.io/badge/status-done-brightgreen) |
| `portainer_policy`            | ![Done](https://img.shields.io/badge/status-done-brightgreen) |
| `portainer_policy_template`   | ![Done](https://img.shields.io/badge/status-done-brightgreen) |
| `portainer_role`              | ![Done](https://img.shields.io/badge/status-done-brightgreen) |
| `portainer_shared_git_credential` | ![Done](https://img.shields.io/badge/status-done-brightgreen) |
| `portainer_user_activity`     | ![Done](https://img.shields.io/badge/status-done-brightgreen) |
| `portainer_helm_git_dryrun`   | ![Done](https://img.shields.io/badge/status-done-brightgreen) |
| `portainer_gitops_repo_refs`  | ![Done](https://img.shields.io/badge/status-done-brightgreen) |
| `portainer_gitops_repo_file`  | ![Done](https://img.shields.io/badge/status-done-brightgreen) |
| `portainer_gitops_source`                 | ![Done](https://img.shields.io/badge/status-done-brightgreen) |
| `portainer_gitops_sources`                | ![Done](https://img.shields.io/badge/status-done-brightgreen) |
| `portainer_gitops_workflow`               | ![Done](https://img.shields.io/badge/status-done-brightgreen) |
| `portainer_gitops_workflows`              | ![Done](https://img.shields.io/badge/status-done-brightgreen) |
| `portainer_gitops_workflow_deployment_resources` | ![Done](https://img.shields.io/badge/status-done-brightgreen) |
| `portainer_gitops_source_connection`      | ![Done](https://img.shields.io/badge/status-done-brightgreen) |
| `portainer_registry_connection` | ![Done](https://img.shields.io/badge/status-done-brightgreen) |
| `portainer_ldap_check`        | ![Done](https://img.shields.io/badge/status-done-brightgreen) |
| `portainer_backup_azure_connection` | ![Done](https://img.shields.io/badge/status-done-brightgreen) |
| `portainer_addons` | ![Done](https://img.shields.io/badge/status-done-brightgreen) |
| `portainer_addon_chart_source` | ![Done](https://img.shields.io/badge/status-done-brightgreen) |
| `portainer_addon_catalog` | ![Done](https://img.shields.io/badge/status-done-brightgreen) |
| `portainer_edge_mtls_ca_certificate` | ![Done](https://img.shields.io/badge/status-done-brightgreen) |
| `portainer_edge_mtls_certificate` | ![Done](https://img.shields.io/badge/status-done-brightgreen) |
| `portainer_endpoint_mtls_certificate` | ![Done](https://img.shields.io/badge/status-done-brightgreen) |
| `portainer_endpoint_mtls_certificate_error` | ![Done](https://img.shields.io/badge/status-done-brightgreen) |
| `portainer_edge_waiting_room` | ![Done](https://img.shields.io/badge/status-done-brightgreen) |
| `portainer_alerting_rule_environments` | ![Done](https://img.shields.io/badge/status-done-brightgreen) |
| `portainer_alerting_rules` | ![Done](https://img.shields.io/badge/status-done-brightgreen) |
| `portainer_auto_updates` | ![Done](https://img.shields.io/badge/status-done-brightgreen) |
| `portainer_omni_machines` | ![Done](https://img.shields.io/badge/status-done-brightgreen) |
| `portainer_omni_machine` | ![Done](https://img.shields.io/badge/status-done-brightgreen) |
| `portainer_omni_machine_logs` | ![Done](https://img.shields.io/badge/status-done-brightgreen) |
| `portainer_omni_talos_versions` | ![Done](https://img.shields.io/badge/status-done-brightgreen) |
| `portainer_omni_upgrade_status` | ![Done](https://img.shields.io/badge/status-done-brightgreen) |
| `portainer_omni_service_account` | ![Done](https://img.shields.io/badge/status-done-brightgreen) |
| `portainer_kubernetes_custom_resource_definitions` | ![Done](https://img.shields.io/badge/status-done-brightgreen) |
| `portainer_kubernetes_custom_resource_definition` | ![Done](https://img.shields.io/badge/status-done-brightgreen) |
| `portainer_kubernetes_custom_resources` | ![Done](https://img.shields.io/badge/status-done-brightgreen) |
| `portainer_kubernetes_custom_resource` | ![Done](https://img.shields.io/badge/status-done-brightgreen) |
| `portainer_kubernetes_gpu` | ![Done](https://img.shields.io/badge/status-done-brightgreen) |
| `portainer_stack_conversion` | ![Done](https://img.shields.io/badge/status-done-brightgreen) |
| `portainer_kubernetes_storage_classes` | ![Done](https://img.shields.io/badge/status-done-brightgreen) |
| `portainer_kubernetes_storage_class` | ![Done](https://img.shields.io/badge/status-done-brightgreen) |
| `portainer_kubernetes_persistent_volume_claims` | ![Done](https://img.shields.io/badge/status-done-brightgreen) |
| `portainer_kubernetes_persistent_volume_claim` | ![Done](https://img.shields.io/badge/status-done-brightgreen) |
| `portainer_kubernetes_volumes` | ![Done](https://img.shields.io/badge/status-done-brightgreen) |
| `portainer_kubernetes_volume` | ![Done](https://img.shields.io/badge/status-done-brightgreen) |
| `portainer_kubernetes_cron_jobs` | ![Done](https://img.shields.io/badge/status-done-brightgreen) |
| `portainer_kubernetes_endpoints` | ![Done](https://img.shields.io/badge/status-done-brightgreen) |
| `portainer_kubernetes_service_account` | ![Done](https://img.shields.io/badge/status-done-brightgreen) |
| `portainer_kubernetes_application` | ![Done](https://img.shields.io/badge/status-done-brightgreen) |
| `portainer_kubernetes_resource_counts` | ![Done](https://img.shields.io/badge/status-done-brightgreen) |
| `portainer_ldap_users` | ![Done](https://img.shields.io/badge/status-done-brightgreen) |
| `portainer_ldap_groups` | ![Done](https://img.shields.io/badge/status-done-brightgreen) |
| `portainer_ldap_admin_groups` | ![Done](https://img.shields.io/badge/status-done-brightgreen) |
| `portainer_ldap_login_test` | ![Done](https://img.shields.io/badge/status-done-brightgreen) |
| `portainer_user_namespaces` | ![Done](https://img.shields.io/badge/status-done-brightgreen) |
| `portainer_current_user_authorizations` | ![Done](https://img.shields.io/badge/status-done-brightgreen) |
| `portainer_licenses_info` | ![Done](https://img.shields.io/badge/status-done-brightgreen) |
| `portainer_recommendations` | ![Done](https://img.shields.io/badge/status-done-brightgreen) |
| `portainer_policy_metadata` | ![Done](https://img.shields.io/badge/status-done-brightgreen) |
| `portainer_policy_conflicts` | ![Done](https://img.shields.io/badge/status-done-brightgreen) |
| `portainer_policy_observability_test` | ![Done](https://img.shields.io/badge/status-done-brightgreen) |
| `portainer_alerting_connectivity` | ![Done](https://img.shields.io/badge/status-done-brightgreen) |
| `portainer_observability_environment` | ![Done](https://img.shields.io/badge/status-done-brightgreen) |
| `portainer_environment_logs` | ![Done](https://img.shields.io/badge/status-done-brightgreen) |
| `portainer_environment_metrics` | ![Done](https://img.shields.io/badge/status-done-brightgreen) |
| `portainer_docker_snapshot` | ![Done](https://img.shields.io/badge/status-done-brightgreen) |
| `portainer_docker_snapshot_containers` | ![Done](https://img.shields.io/badge/status-done-brightgreen) |
| `portainer_docker_snapshot_container` | ![Done](https://img.shields.io/badge/status-done-brightgreen) |
| `portainer_image_status` | ![Done](https://img.shields.io/badge/status-done-brightgreen) |
| `portainer_edge_update_schedule_info` | ![Done](https://img.shields.io/badge/status-done-brightgreen) |
| `portainer_agent_versions` | ![Done](https://img.shields.io/badge/status-done-brightgreen) |
| `portainer_edge_update_previous_versions` | ![Done](https://img.shields.io/badge/status-done-brightgreen) |
| `portainer_edge_update_schedules_active` | ![Done](https://img.shields.io/badge/status-done-brightgreen) |
| `portainer_edge_configuration_files` | ![Done](https://img.shields.io/badge/status-done-brightgreen) |
| `portainer_edge_stack_stagger_status` | ![Done](https://img.shields.io/badge/status-done-brightgreen) |
| `portainer_gitops_repo_file_search` | ![Done](https://img.shields.io/badge/status-done-brightgreen) |
| `portainer_gitops_helm_values` | ![Done](https://img.shields.io/badge/status-done-brightgreen) |
| `portainer_system`                        | ![Done](https://img.shields.io/badge/status-done-brightgreen) |
| `portainer_settings_public`               | ![Done](https://img.shields.io/badge/status-done-brightgreen) |
| `portainer_motd`                          | ![Done](https://img.shields.io/badge/status-done-brightgreen) |
| `portainer_user_access`                   | ![Done](https://img.shields.io/badge/status-done-brightgreen) |
| `portainer_kubernetes_cluster`            | ![Done](https://img.shields.io/badge/status-done-brightgreen) |
| `portainer_kubernetes_nodes`              | ![Done](https://img.shields.io/badge/status-done-brightgreen) |
| `portainer_kubernetes_events`             | ![Done](https://img.shields.io/badge/status-done-brightgreen) |
| `portainer_kubernetes_describe`           | ![Done](https://img.shields.io/badge/status-done-brightgreen) |
| `portainer_kubernetes_persistent_volumes` | ![Done](https://img.shields.io/badge/status-done-brightgreen) |
| `portainer_kubernetes_config`             | ![Done](https://img.shields.io/badge/status-done-brightgreen) |
| `portainer_docker_dashboard`              | ![Done](https://img.shields.io/badge/status-done-brightgreen) |
| `portainer_docker_images`                 | ![Done](https://img.shields.io/badge/status-done-brightgreen) |
| `portainer_docker_container_gpus`         | ![Done](https://img.shields.io/badge/status-done-brightgreen) |
| `portainer_app_templates`                 | ![Done](https://img.shields.io/badge/status-done-brightgreen) |
| `portainer_helm_chart`                    | ![Done](https://img.shields.io/badge/status-done-brightgreen) |
| `portainer_custom_template_file`          | ![Done](https://img.shields.io/badge/status-done-brightgreen) |
| `portainer_edge_stack_file`               | ![Done](https://img.shields.io/badge/status-done-brightgreen) |
| `portainer_edge_job_file`                 | ![Done](https://img.shields.io/badge/status-done-brightgreen) |
| `portainer_edge_job_tasks`                | ![Done](https://img.shields.io/badge/status-done-brightgreen) |
| `portainer_edge_job_task_logs`            | ![Done](https://img.shields.io/badge/status-done-brightgreen) |
| `portainer_endpoints_summary`             | ![Done](https://img.shields.io/badge/status-done-brightgreen) |
| `portainer_endpoint_registries`           | ![Done](https://img.shields.io/badge/status-done-brightgreen) |
| `portainer_team_memberships`              | ![Done](https://img.shields.io/badge/status-done-brightgreen) |
| `portainer_kubernetes_pod_metrics`        | ![Done](https://img.shields.io/badge/status-done-brightgreen) |
| `portainer_kubernetes_node_metrics`       | ![Done](https://img.shields.io/badge/status-done-brightgreen) |
| `portainer_kubernetes_application_resources` | ![Done](https://img.shields.io/badge/status-done-brightgreen) |
| `portainer_helm_release_history` | ![Done](https://img.shields.io/badge/status-done-brightgreen) |


### 🐳 Podman Support via Docker Resources

[Podman is compatible with the Docker API](https://docs.podman.io/en/latest/_static/api.html), which means you can use existing `portainer_docker_*` resources with Podman – **no special `portainer_podman_*` resources are needed**.

✅ **Use Docker resources for Podman**  
Podman works out of the box with most `portainer_docker_*` Terraform resources.

> ⚠️ **Note**:  
> Podman **does not support Docker Swarm** – any swarm-based features are **not compatible**.

### Docker to Kubernetes with D2K

The `portainer_d2k_environment` resource (Portainer Business Edition 3.0 or later) installs d2k in a namespace of a Kubernetes environment and registers a Docker (or Docker Swarm) environment whose Docker API d2k serves on the cluster, so the existing `portainer_docker_*` and `portainer_stack` resources deploy Docker workloads onto Kubernetes directly.

It replaces the removed `portainer_compose_convert` (Kompose) resource. To convert an existing stack into Kubernetes manifests or a Helm chart instead, use the `portainer_stack_conversion` data source (Business Edition).
See full documentation: [docs/resources/d2k_environment.md](https://github.com/portainer/terraform-provider-portainer/tree/main/docs/resources/d2k_environment.md)

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
- HashiCorp Terraform
- [Portainer](https://portainer.io)
- [OpenTofu](https://opentofu.org/)
- [Docker](https://www.docker.com/)
