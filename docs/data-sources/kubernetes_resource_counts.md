# Data Source Documentation: `portainer_kubernetes_resource_counts`

# portainer_kubernetes_resource_counts

Reports how much is in a Kubernetes cluster: applications, namespaces, services, ingresses, ConfigMaps, secrets and volumes.

## Example Usage

```hcl
data "portainer_kubernetes_resource_counts" "prod" {
  endpoint_id = portainer_environment.prod.id
}

output "cluster_size" {
  value = data.portainer_kubernetes_resource_counts.prod
}
```

## Arguments Reference

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `endpoint_id` | number | ✅ yes | Identifier of the Kubernetes environment to query. |

## Attributes Reference

| Name | Type | Description |
|------|------|-------------|
| `applications` | number | Applications in the cluster. |
| `config_maps` | number | ConfigMaps in the cluster. |
| `ingresses` | number | Ingresses in the cluster. |
| `namespaces` | number | Namespaces in the cluster. |
| `secrets` | number | Secrets in the cluster. |
| `services` | number | Services in the cluster. |
| `volumes` | number | Volumes in the cluster. |

Portainer answers each of these from its own endpoint, so one read makes seven calls. They are folded into a single data source because they answer a single question; pulling them apart would mean seven data sources that are each one number.
