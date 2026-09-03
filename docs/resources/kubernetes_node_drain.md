# Resource Documentation: `portainer_kubernetes_node_drain`

# portainer_kubernetes_node_drain
The `portainer_kubernetes_node_drain` resource drains a Kubernetes node through Portainer, evicting its pods so the node can be taken out of service for maintenance. Every option mirrors the equivalent `kubectl drain` flag.

Draining is a one-shot operation, so this resource behaves like an action: it runs on create, has nothing to read back, and removing it from the configuration only drops it from state — the node is **not** uncordoned. Changing any argument replaces the resource, which drains again.

Requires Portainer **2.45.0** or newer for the advanced options (`force`, `timeout_seconds`, `grace_period_seconds`, `ignore_daemon_sets`, `delete_empty_dir_data`, `disable_eviction`); earlier versions accept the drain request but ignore the payload.

## Example Usage

```hcl
resource "portainer_kubernetes_node_drain" "maintenance" {
  environment_id = 1
  node_name      = "worker-1"

  ignore_daemon_sets    = true
  delete_empty_dir_data = true
  timeout_seconds       = 300
  grace_period_seconds  = 30
  force                 = false
  disable_eviction      = false
}
```

## Arguments Reference

| Name                    | Type   | Required | Default | Description                                                                                                                                     |
|-------------------------|--------|----------|---------|-------------------------------------------------------------------------------------------------------------------------------------------------|
| `environment_id`        | number | Yes      | –       | Identifier of the Portainer Kubernetes environment the node belongs to.                                                                          |
| `node_name`             | string | Yes      | –       | Name of the Kubernetes node to drain.                                                                                                            |
| `force`                 | bool   | No       | `false` | Allow deletion of standalone pods not managed by a controller. Such pods are lost for good, so the drain refuses them unless this is enabled.     |
| `timeout_seconds`       | number | No       | `60`    | Overall time in seconds to wait for the drain to complete.                                                                                       |
| `grace_period_seconds`  | number | No       | `-1`    | Termination grace period applied to every evicted pod. `-1` keeps each pod's own grace period.                                                    |
| `ignore_daemon_sets`    | bool   | No       | `true`  | Skip DaemonSet-managed pods, which would otherwise block the drain because their controller recreates them immediately.                          |
| `delete_empty_dir_data` | bool   | No       | `true`  | Evict pods that use an emptyDir volume, whose data is lost once the pod is deleted.                                                              |
| `disable_eviction`      | bool   | No       | `false` | Delete pods directly instead of going through the eviction API, which ignores any configured PodDisruptionBudget.                                |

The defaults above are Portainer's own (`libkubectl.DefaultDrainOptions`), so leaving a field unset behaves exactly like omitting it from the API request.

## Attributes Reference

| Name | Description                                                            |
|------|------------------------------------------------------------------------|
| `id` | Composite identifier in the form `<environment_id>/<node_name>`.       |
