---
page_title: "Upgrading to v3.0.0"
subcategory: ""
description: |-
  What changes in v3.0.0 of the Portainer provider and how to update a configuration.
---

# Upgrading to v3.0.0

v3.0.0 removes resources and attributes that had no effect or that Portainer no longer supports, makes one secret write-only, and uses one import ID format for every resource. It adds support for Portainer Business Edition 3.0.

Most configurations upgrade without changes. Run `terraform plan` after upgrading: every item below either shows up as an error in `validate`/`plan`, or needs no change at all.

Write-only attributes still need Terraform 1.11 or later (or OpenTofu 1.11 or later). The provider is tested daily against Portainer CE 2.45 LTS and 2.39 LTS, and on every pull request and weekly against Portainer Business Edition 2.45.3 and 3.0.1. Features that only Portainer Business Edition 3.0 has say so in their documentation, and fail with an error that names the requirement when the server does not meet it.

## Removed resources

### `portainer_compose_convert`

The resource ran the `kompose` binary on the machine running Terraform and kept nothing in state. Use one of these instead:

- **D2K** (Portainer Business Edition 3.0): runs Docker workloads on Kubernetes without converting them. `portainer_d2k_environment` adds a D2K environment to a Kubernetes environment, and the existing Docker resources work against it.
- **`portainer_stack_conversion`** (Business Edition): converts an existing stack to Kubernetes manifests or a Helm chart through Portainer.
- Run `kompose convert` outside Terraform, for example in CI, and deploy the result with `portainer_kubernetes_application`.

Remove the resource from the configuration. Since it had no state worth keeping, `terraform state rm portainer_compose_convert.<name>` removes it from state without touching anything else.

### OpenAMT: `portainer_open_amt`, `portainer_open_amt_activate`, `portainer_open_amt_devices_action`, `portainer_open_amt_devices_features`

Portainer removed Intel OpenAMT support; the API no longer has these endpoints. Remove the resources from the configuration and from state:

```shell
terraform state rm portainer_open_amt.<name>
```

## Changed resources

### `portainer_settings`

| Attribute | Change | What to do |
|-----------|--------|------------|
| `enable_telemetry` | Removed. Portainer has no telemetry setting. | Remove it from the configuration. |
| `display_donation_header` | Read-only. Portainer never accepted it through the settings API, so a configured value had no effect. | Remove it from the configuration; it can still be read. |
| `display_external_contributors` | Read-only, as above. | Remove it from the configuration. |
| `is_docker_desktop_extension` | Read-only. Portainer detects it at startup. | Remove it from the configuration. |

All four were deprecated in v2.1.0.

### `portainer_restore`: `setup_token` is write-only

The setup token is a one-time bootstrap secret, so it no longer has a plaintext attribute that stores it in state.

```hcl
# Before
resource "portainer_restore" "bootstrap" {
  # ...
  setup_token = var.setup_token
}

# After
resource "portainer_restore" "bootstrap" {
  # ...
  setup_token_wo         = var.setup_token
  setup_token_wo_version = 1
}
```

`portainer_restore` runs once on creation, so the change does not replace an existing restore.

### `portainer_alerting_rule`: destroy only deletes rules it created

`rule_id` is now optional. Without it the resource creates a custom alert rule; with it, the resource adopts an existing rule as before.

Destroy now deletes a rule from Portainer only when this resource created it (`created = true`). An adopted or imported rule, including every rule managed with v2, is only removed from Terraform state and stays in Portainer. v2 sent `DELETE` for every rule, including Portainer's predefined ones. To switch a predefined rule off, set `enabled = false`.

Updates now read the rule first and send it back whole with your changes, so attributes you do not set are kept.

### `portainer_docker_volume`: `driver` defaults to the engine's

`driver` no longer defaults to `local`. Left unset, the engine chooses: `local` on Docker, `d2k` on a D2K environment, where the old default replaced the volume on every plan. Existing volumes plan no change.

## Import IDs

Every resource with a composite import ID now separates the parts with `:`. The old formats are still accepted, so existing `import` blocks and scripts keep working.

| Resource | v3 format | Accepted old format |
|----------|-----------|---------------------|
| `portainer_stack` | `<endpoint_id>:<stack_id>:<deployment_type>[:<method>]` (`method` is required unless the stack was deployed from Git) | `<endpoint_id>-<stack_id>-<deployment_type>[-<method>]` |
| `portainer_docker_volume` | `<endpoint_id>:<volume_name>` | `<endpoint_id>-<volume_name>` |
| `portainer_docker_config` | `<endpoint_id>:<config_id>` | `<endpoint_id>-<config_id>` |
| `portainer_docker_secret` | `<endpoint_id>:<secret_id>` | `<endpoint_id>-<secret_id>` |
| `portainer_docker_node` | `<endpoint_id>:<node_id>` | `<endpoint_id>-<node_id>` |
| `portainer_registry_access` | `<registry_id>:<endpoint_id>:team:<team_id>` or `...:user:<user_id>` | `<registry_id>/<endpoint_id>/team/<team_id>` |
| `portainer_endpoint_group_access` | `<endpoint_group_id>:team:<team_id>` or `...:user:<user_id>` | `<endpoint_group_id>/team/<team_id>` |
| `portainer_kubernetes_persistent_volume` | `<environment_id>:<name>` | `<environment_id>/<name>` |
| `portainer_omni_cluster` | `<credential_id>:<cluster_name>` | `<credential_id>/<cluster_name>` |

For `portainer_docker_volume`, `portainer_docker_node`, `portainer_registry_access`, `portainer_endpoint_group_access`, `portainer_kubernetes_persistent_volume` and `portainer_omni_cluster` the resource ID itself uses the same format. Resources created with v3 get the new format. Existing ones keep working with their old ID: `portainer_docker_volume`, `portainer_docker_node` and `portainer_omni_cluster` move to the new format on the next refresh, `portainer_endpoint_group_access` and `portainer_registry_access` on their next update. Nothing is replaced, but a configuration that uses one of these `id` values elsewhere sees that value change once. The IDs of `portainer_stack`, `portainer_docker_config` and `portainer_docker_secret` are the object ID alone and do not change.

## New in v3.0.0

All of these need Portainer Business Edition 3.0 or later unless noted:

- `portainer_d2k_environment`: a D2K environment (Docker API on Kubernetes) on a Kubernetes environment.
- KubeSolo: `portainer_kubesolo_offline_bundle`, and the data sources `portainer_kubesolo_versions`, `portainer_kubesolo_offline_bundles` and `portainer_kubesolo_config`. Hosts are installed outside Terraform; `examples/kubesolo_edge` shows cloud-init with an edge key from `portainer_environment`.
- GitOps reconcile engine on `portainer_gitops_workflow` (`engine`, `reconcile_interval`, environment and environment group targets, dependencies, Helm, prune, force apply, drift mode), the action `portainer_gitops_workflow_reconcile` and the data source `portainer_gitops_workflow_deployment_resources`. Existing workflows plan no change.
- Kubernetes Gateway API: `portainer_kubernetes_gateway`, `portainer_kubernetes_gateway_http_route` and the data source `portainer_kubernetes_gateway_classes`.
- `portainer_kubernetes_scoped_kubeconfig` (Business Edition 2.45.2 or later) and the data source `portainer_kubernetes_scoped_kubeconfigs` (Business Edition 3.0).
- Data source `portainer_kubernetes_namespace_access`.
- Kubernetes options: `kubernetes_prune` and `kubernetes_force_apply` on `portainer_stack` and `portainer_edge_stack` (Kubernetes deployments only).
- `portainer_kubernetes_namespace` reports every resource quota of the namespace (`resource_quotas`) and the effective limit per resource (`effective_ceiling`).
- `portainer_environment`: `delete_cluster_on_destroy` uninstalls KubeSolo from a machine Portainer installed it on when the environment is destroyed.
- `portainer_user` (resource and data source): `created_authentication_method` and `last_authentication_method`.
- `portainer_policy` accepts the 3.0 types `vulnerability-scanning-k8s` and `automatic-namespace-provisioning-k8s`, and the 2.45 types it was missing.
- Add-ons: data source `portainer_addon_catalog` and the action `portainer_addon_catalog_refresh` (Business Edition 2.45.2 or later).
- Alerting: custom rules with `portainer_alerting_rule` and the data source `portainer_alerting_rules` (Business Edition), and `portainer_observability_environment` (Business Edition 2.45 or later).
- `portainer_system` has two new attributes: `addons_available` and `addons_availability` (the reason, for example `kubernetes-required`). Portainer 3.0 reports availability as a string, 2.45 as a boolean; both are read.

The Portainer AI products (Portainer-Run, Portainer-Command, Portainer-IDP and others) are Business Edition add-ons: install and configure them with `portainer_addon`, `portainer_addon_access` and `portainer_addon_config`.

## Portainer 3.0 behavior the provider follows

- Stack Git webhooks answer `202` with a JSON body instead of an empty `200`; `portainer_stack_webhook` accepts both.
- The S3 backup settings no longer return the secret access key and the backup password; `portainer_backup_s3` keeps the configured values instead of planning a change on every run.
- `/system/info` reports `addonsAvailable` as a string; `portainer_system` reads both forms.
- The access endpoint of a GitOps source returns a different body; `portainer_gitops_source` reads the source again instead of decoding it.
