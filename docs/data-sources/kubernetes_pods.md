# Data Source Documentation: `portainer_kubernetes_pods`

# portainer_kubernetes_pods
The `portainer_kubernetes_pods` data source lists Kubernetes pods from a Portainer-managed environment, either across the whole cluster or within a single namespace, with optional label and field selectors.

The cluster-wide form requires Portainer **2.45.0** or newer; the namespaced form works with earlier versions too.

## Example Usage

### Find pods that are not running

```hcl
data "portainer_kubernetes_pods" "not_running" {
  environment_id = 1
  field_selector = "status.phase!=Running"
}

output "unhealthy_pods" {
  value = [
    for p in data.portainer_kubernetes_pods.not_running.pods : "${p.namespace}/${p.name} (${p.phase})"
  ]
}
```

### Check that an application's pods are all ready

```hcl
data "portainer_kubernetes_pods" "web" {
  environment_id = 1
  namespace      = "default"
  label_selector = "app=web"
}

output "web_pods_ready" {
  value = alltrue([for p in data.portainer_kubernetes_pods.web.pods : p.ready])
}
```

## Arguments Reference

| Name             | Type   | Required | Description                                                                                                    |
|------------------|--------|----------|----------------------------------------------------------------------------------------------------------------|
| `environment_id` | number | Yes      | Environment (endpoint) identifier of the Kubernetes environment to query.                                       |
| `namespace`      | string | No       | Namespace to list pods from. Leave unset to list pods across every namespace the API token can access.          |
| `label_selector` | string | No       | Kubernetes label selector used to filter the pods (for example `app=nginx`).                                    |
| `field_selector` | string | No       | Kubernetes field selector used to filter the pods (for example `status.phase=Running`).                          |

## Attributes Reference

| Name   | Type | Description                                                                                                                                                                                                          |
|--------|------|----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `pods` | list | Pods matching the query. Each entry has `name`, `namespace`, `node_name`, `phase`, `pod_ip`, `host_ip`, `service_account`, `labels`, `ready`, `restart_count`, `start_time` and `containers` (each with `name`, `image`, `ready`, `restart_count`). |

`ready` is true only when every container in the pod reports ready, and false while the kubelet has not reported container statuses yet. `restart_count` on the pod is the sum across its containers.
