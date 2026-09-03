# Data Source Documentation: `portainer_kubernetes_cluster`

# portainer_kubernetes_cluster
Describes a Kubernetes cluster behind a Portainer environment: its version, whether RBAC is on, what Portainer counts in it, and how much room is left for a new workload.

## Example Usage

```hcl
data "portainer_kubernetes_cluster" "prod" {
  environment_id = 1
}

# RBAC-dependent resources are pointless without it.
resource "portainer_kubernetes_namespace_access" "team" {
  count = data.portainer_kubernetes_cluster.prod.rbac_enabled ? 1 : 0
  # ...
}
```

## Arguments Reference

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `environment_id` | number | ✅ yes | Environment (endpoint) identifier of the Kubernetes environment. |

## Attributes Reference

| Name | Type | Description |
|------|------|-------------|
| `version` | string | Full Kubernetes version reported by the API server. |
| `major` | string | Major version component. |
| `minor` | string | Minor version component. |
| `platform` | string | Platform the API server runs on, such as `linux/amd64`. |
| `supports_pod_restart` | bool | Whether the cluster is new enough for Portainer's pod restart action. |
| `rbac_enabled` | bool | Whether RBAC is enabled. Roles, role bindings and namespace access policies only take effect when it is. |
| `applications_count` | number | Applications Portainer sees in the cluster. |
| `namespaces_count` | number | Namespaces visible to the API token. |
| `services_count` | number | Services in the cluster. |
| `ingresses_count` | number | Ingresses in the cluster. |
| `config_maps_count` | number | ConfigMaps in the cluster. |
| `secrets_count` | number | Secrets in the cluster. |
| `volumes_count` | number | Volumes in the cluster. |
| `max_cpu` | number | Largest CPU allocation a single workload could still request, in millicores. |
| `max_memory` | number | Largest memory allocation a single workload could still request, in bytes. |

The counters come from Portainer's dashboard endpoint and reflect what the API token can see, not necessarily the whole cluster.
