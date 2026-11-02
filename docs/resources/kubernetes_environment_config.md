# ☸️ **Resource Documentation: `portainer_kubernetes_environment_config`**

# portainer_kubernetes_environment_config

The `portainer_kubernetes_environment_config` resource manages the Kubernetes features configuration of a Kubernetes environment, the settings the Portainer UI shows under **Environments → (cluster) → Kubernetes configuration**: load balancers, metrics, resource over-commit, access to the `default` namespace and the storage classes users may pick.

Ingress classes are not part of this resource; manage them with [`portainer_kubernetes_ingresscontrollers`](kubernetes_ingresscontrollers.md).

## Example Usage

```hcl
resource "portainer_kubernetes_environment_config" "cluster" {
  endpoint_id = 4

  use_load_balancer               = true
  use_server_metrics              = true
  enable_resource_over_commit     = false
  resource_over_commit_percentage = 80
  restrict_default_namespace      = true

  storage_class {
    name                   = "local-path"
    provisioner            = "rancher.io/local-path"
    access_modes           = ["RWO"]
    allow_volume_expansion = false
  }
}
```

## Lifecycle & Behavior

- Portainer stores this configuration with the environment and replaces it as a whole on every update. The provider reads the current configuration first and changes only the managed settings, so snapshots, detected cluster features and ingress classes are kept.
- Every argument is optional. One that is left out keeps the value configured in Portainer, and the resource reads all of them back, so changes made in the Portainer UI show up in the plan.
- `storage_class` blocks replace the list of enabled storage classes when at least one is set.
- Destroying the resource only removes it from state; the environment keeps its configuration.

## Arguments Reference

| Name                                 | Type   | Required    | Description                                                                                   |
|--------------------------------------|--------|-------------|-----------------------------------------------------------------------------------------------|
| `endpoint_id`                        | number | ✅ yes      | ID of the Kubernetes environment. Changing it forces recreation                                |
| `use_load_balancer`                  | bool   | 🚫 optional | Allow users to expose applications through a LoadBalancer service                              |
| `use_server_metrics`                 | bool   | 🚫 optional | Read metrics from the cluster's metrics server                                                 |
| `enable_resource_over_commit`        | bool   | 🚫 optional | Allow namespace quotas to add up to more than the cluster's capacity                           |
| `resource_over_commit_percentage`    | number | 🚫 optional | Share of the cluster (1-100) namespaces may reserve when over-commit is disabled               |
| `restrict_default_namespace`         | bool   | 🚫 optional | Restrict the `default` namespace to administrators                                             |
| `ingress_availability_per_namespace` | bool   | 🚫 optional | Allow ingress classes to be made available per namespace                                       |
| `allow_none_ingress_class`           | bool   | 🚫 optional | Allow ingresses without an ingress class                                                       |
| `storage_class`                      | block  | 🚫 optional | Storage classes users may pick (see below)                                                     |

### `storage_class` Block

| Name                     | Type         | Required    | Description                                    |
|--------------------------|--------------|-------------|------------------------------------------------|
| `name`                   | string       | ✅ yes      | Name of the Kubernetes StorageClass            |
| `provisioner`            | string       | ✅ yes      | Provisioner of the StorageClass                |
| `access_modes`           | list(string) | 🚫 optional | Access modes users may request (`RWO`, `RWX`)  |
| `allow_volume_expansion` | bool         | 🚫 optional | Allow volumes of this class to be expanded     |

## Attributes Reference

| Name | Description                    |
|------|--------------------------------|
| `id` | ID of the Kubernetes environment |

## Import

The configuration can be imported using the numeric environment ID:

```shell
terraform import portainer_kubernetes_environment_config.cluster 4
```
