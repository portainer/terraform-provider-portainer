---
page_title: "Upgrading to v3.1.0"
subcategory: ""
description: |-
  What changes in v3.1.0 of the Portainer provider: the move to terraform-plugin-framework, new ephemeral resources, actions, list resources and resource identity.
---

# Upgrading to v3.1.0

v3.1.0 is a minor release: configurations, state and import IDs written for v3.0.0 keep working without changes. Upgrade the version constraint, run `terraform init -upgrade` and `terraform plan`; the plan should be empty.

## What changed under the hood

The provider is now built on [terraform-plugin-framework](https://developer.hashicorp.com/terraform/plugin/framework) instead of Plugin SDK v2. Every resource and data source keeps its arguments, blocks, defaults, import ID formats and state layout; the test suite applies each resource with v3.0.0 and checks that v3.1.0 plans no change.

The provider still speaks protocol 5, so every Terraform and OpenTofu version that ran v3.0.0 runs v3.1.0.

## Plans that may show a one-time change

A few states written by v3.0.0 cannot be told apart from a different configuration. In these cases the first plan after the upgrade shows an in-place update that sends Portainer what it already has, and later plans are empty:

- **`portainer_gitops_source`** whose `url` is written differently from how Portainer stores it (for example with a trailing `.git`): the update records the configured spelling and calls no Portainer API.
- **Arguments set explicitly to `false` or `0`** where v3.0.0 stored the same value for "not set" (for example the artifact arguments of `portainer_gitops_workflow`): the update sends the unchanged value.
- **`portainer_docker_network` with `enable_ipv4` or `enable_ipv6` left out**: on Docker 28 and later, v3.0.0 read `enable_ipv4 = true` back and planned a replacement on every run. v3.1.0 leaves the flags alone when the configuration does not set them; a network created under v3.0.0 may be replaced once more.

v3.0.0 crashed reading a `portainer_docker_network` whose IPAM configuration has a gateway (`Invalid address to set: ipam_config.0.Gateway`). v3.1.0 reads such networks.

v3.0.0 planned `file_path_in_repository = "docker-compose.yml" -> ""` on every run for a `portainer_stack` deployed from Git without that argument; v3.1.0 keeps the path Portainer reports. Creating a `portainer_edge_stack` with `pull_image = true` no longer fails with "inconsistent result after apply".

## Deprecations

Live tests against current Portainer releases showed that a few resources call APIs Portainer has removed. They keep working against the releases that still have the API, warn when used, and will be removed in the next major version:

- **`portainer_user_git_credential`**: per-user git credentials exist only in Business Edition 2.42 and earlier. Use `portainer_shared_git_credential`, which Business Edition 3.1 requires to have a `url_pattern`.
- **`portainer_cloud_provider_provision`**: the cloud provisioning API exists only in Business Edition 2.39 LTS and 2.41 to 2.43.
- **`portainer_chat`** and the `openai_integration` argument of `portainer_settings_experimental`: Portainer dropped its OpenAI integration. `openai_integration` is no longer read back, so it no longer plans a change on every run.

On a server without the API, these fail with an error that names the releases that have it.

## Error messages

Validation messages come from the framework now and are worded differently, for example a missing required block reads `Block listener must have a configuration value as the provider has marked it as required` instead of `Insufficient listener blocks`. Scripts that match error texts may need updating.

## New features

These need newer Terraform versions; the resources and data sources of v3.0.0 work as before on older ones.

| Feature | Terraform | OpenTofu |
|---------|-----------|----------|
| Ephemeral resources | 1.10 | 1.11 |
| Import by resource identity | 1.12 | not supported |
| Actions | 1.14 | not supported |
| List resources (`terraform query`) | 1.14 | not supported |

### Ephemeral resources

Ephemeral resources produce a secret for the duration of one Terraform run without writing it to the plan or the state: `portainer_user_api_key`, `portainer_auth`, `portainer_kubernetes_config`, `portainer_kubernetes_scoped_kubeconfig`, `portainer_endpoints_edge_generate_key` and `portainer_sshkeygen`. Where Portainer creates something server-side (an API key, a scoped kubeconfig, a session), it is removed again when the run ends.

### Actions

The one-shot resources such as `portainer_endpoint_snapshot`, `portainer_backup_local_run`, `portainer_webhook_execute` or `portainer_kubernetes_deployment_scale` are also available as actions of the same name. An action runs when you invoke it, from a `lifecycle { action_trigger { ... } }` block or with `terraform apply -invoke action.portainer_endpoint_snapshot.<name>`, and leaves nothing in state. The resources stay for OpenTofu and older Terraform.

### List resources and resource identity

`portainer_tag`, `portainer_team`, `portainer_user`, `portainer_endpoint_group`, `portainer_edge_group`, `portainer_environment`, `portainer_registry`, `portainer_custom_template`, `portainer_docker_network`, `portainer_docker_volume`, `portainer_docker_secret`, `portainer_docker_config`, `portainer_docker_plugin`, `portainer_docker_node`, `portainer_stack` and `portainer_webhook` can be listed with `terraform query`, which can also generate the configuration and import blocks for what it finds. The same resources can be imported with an `identity` instead of an ID string:

```terraform
import {
  to       = portainer_tag.prod
  identity = { id = "5" }
}
```

The Docker resources and `portainer_stack` have composite identities that mirror their import IDs, e.g. `{ endpoint_id = 1, id = "a1b2c3" }` for a network or `{ endpoint_id = 1, id = "5", deployment_type = "standalone" }` for a stack (whose `method` an import by identity infers: `repository` for Git stacks, otherwise `string`).

The Kubernetes manifest resources (`portainer_kubernetes_application`, `portainer_kubernetes_cronjob`, `portainer_kubernetes_job`, `portainer_kubernetes_serviceaccounts`, `portainer_kubernetes_configmaps`, `portainer_kubernetes_secret`, `portainer_kubernetes_service`, `portainer_kubernetes_role`, `portainer_kubernetes_rolebinding`, `portainer_kubernetes_clusterrole`, `portainer_kubernetes_clusterrolebinding`, `portainer_kubernetes_volume`, `portainer_kubernetes_storage`) and `portainer_edge_stack`, `portainer_edge_job`, `portainer_edge_configurations`, `portainer_gitops_source` and `portainer_gitops_workflow` have identities and list resources too. A Kubernetes object's identity is the parts of its import ID, e.g. `{ endpoint_id = 1, namespace = "default", name = "web" }`; its list resource needs `endpoint_id` and fills `manifest` from the live object with `include_resource = true`.

### New arguments

v3.1.0 covers every request field of the Portainer API (Business Edition 3.1.0, 3.0.1 and 2.45.4, Community Edition 2.45.4) that a configuration can set. New, all optional:

- `portainer_environment`: Azure credentials, Edge Agent intervals and async mode, tunnel server address, and the custom template of a KaaS environment.
- `portainer_edge_stack`: `stagger_config`, per-device configurations; `pull_image` now also re-pulls on redeploy.
- `portainer_stack`: `pull_image` re-pulls images on redeploy; `portainer_stack_migrate`: `is_helm`.
- `portainer_custom_template`: `edge_settings`.
- `portainer_settings`: `custom_login_banner`, `auto_patch_settings`, `edge`; `portainer_settings_experimental`: `fleet_management`.
- `portainer_team`: `deny_portainer_access`; `portainer_shared_git_credential`: `url_pattern`.
- `portainer_kubernetes_namespace`: `storage_quotas`, `load_balancer_quota`.

### Portainer Business Edition 3.1

v3.1.0 supports Portainer Business Edition 3.1 and is tested on every pull request and weekly against Business Edition 2.45.4, 3.0.1 and 3.1.0, and daily against Community Edition 2.45.4 and 2.39 LTS. The 3.1 arguments are only sent when configured, so older servers are unaffected:

- `portainer_addon`: `public_url`, set without reinstalling the addon (add-ons need Portainer itself running on Kubernetes).
- `portainer_backup_local_settings`: `password` / `password_wo` to encrypt the archives. In FIPS mode Portainer 3.1 refuses unencrypted backups, so set a password on the local, S3 and Azure backups there.
- `portainer_stack`: `atomic` rolls back a failed Helm chart deployment.
- Portainer's built-in OIDC provider: `portainer_settings` block `oidc_provider` (public URL, token and session lifetimes), the resources `portainer_oidc_client` (with identity and list resource) and `portainer_oidc_client_secret` (regenerated in place with `regenerate_version`), the actions and action resources `portainer_oidc_keys_rotate`, `portainer_oidc_client_sessions_end` and `portainer_oidc_user_sessions_end`, and the data sources `portainer_oidc_clients`, `portainer_oidc_client_sessions` and `portainer_oidc_launcher`. Portainer 3.1.0 serves the OIDC provider only when started with the `oidc` feature flag (`--feat=oidc`).
- GitOps workflow artifacts accept glob file patterns, and `portainer_kubesolo_config` returns the Edge key redacted.
- Non-administrator API keys act on Kubernetes through the user's own ServiceAccount: run the provider with an administrator's API key.

### Drift detection

- `portainer_kubernetes_ingresses` reads hosts, paths, TLS, class and annotations back from the cluster, so changes made outside Terraform show up in the plan. Import now reads every argument except `labels`.
- `portainer_kubernetes_gateway` manages only the annotation and label keys in the configuration; keys added by controllers or other tools are kept.
- `portainer_alerting_settings` import reads the notification channels again.
