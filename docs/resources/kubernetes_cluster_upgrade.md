# Resource Documentation: `portainer_kubernetes_cluster_upgrade`

# portainer_kubernetes_cluster_upgrade

> **Business Edition only.** This endpoint does not exist in Portainer CE.


Upgrades a provisioned Kubernetes cluster to the next stable version.

## Example Usage

```hcl
resource "portainer_kubernetes_cluster_upgrade" "example" {
  endpoint_id = portainer_environment.prod.id

  triggers = {
    run_at = timestamp()
  }
}
```

## Arguments Reference

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `endpoint_id` | number | ✅ yes | Identifier of the provisioned Kubernetes environment to upgrade. |
| `triggers` | map | ❌ no | Arbitrary values that force another run when they change. |

This is a one-shot action with no state to read back, so without a `triggers` value the resource runs once and then stays put.
