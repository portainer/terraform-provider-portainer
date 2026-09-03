# Data Source Documentation: `portainer_kubernetes_replicasets`

# portainer_kubernetes_replicasets
The `portainer_kubernetes_replicasets` data source lists Kubernetes replica sets in a namespace. Its main use is discovering the rollout history of a deployment: each replica set carries the revision it represents, which is exactly what `portainer_kubernetes_deployment_rollback` takes as its target.

Requires Portainer **2.45.0** or newer.

## Example Usage

```hcl
data "portainer_kubernetes_replicasets" "web" {
  environment_id = 1
  namespace      = "default"
  deployment     = "web"
}

output "web_revisions" {
  value = {
    for rs in data.portainer_kubernetes_replicasets.web.replicasets : rs.name => rs.revision
  }
}
```

## Arguments Reference

| Name             | Type   | Required | Description                                                                                                       |
|------------------|--------|----------|---------------------------------------------------------------------------------------------------------------------|
| `environment_id` | number | Yes      | Environment (endpoint) identifier of the Kubernetes environment to query.                                          |
| `namespace`      | string | Yes      | Namespace whose replica sets are listed.                                                                           |
| `deployment`     | string | No       | Only return replica sets owned by this deployment.                                                                 |
| `label_selector` | string | No       | Kubernetes label selector used to filter the replica sets (for example `app=nginx`).                                |
| `field_selector` | string | No       | Kubernetes field selector used to filter the replica sets (for example `metadata.name=web-1234`).                    |

## Attributes Reference

| Name          | Type | Description                                                                                                                                                                                    |
|---------------|------|--------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `replicasets` | list | Replica sets matching the query. Each entry has `name`, `namespace`, `revision`, `labels`, `images`, `replicas`, `ready_replicas`, `available_replicas`, `fully_labeled_replicas` and `creation_timestamp`. |

`revision` comes from the `deployment.kubernetes.io/revision` annotation and is `0` for a replica set that no deployment owns.
