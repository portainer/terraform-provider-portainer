# Data Source Documentation: `portainer_kubernetes_endpoints`

# portainer_kubernetes_endpoints

> **Business Edition only.** This endpoint does not exist in Portainer CE.


Lists the Kubernetes Endpoints objects in a cluster - the addresses behind each service.

## Example Usage

```hcl
data "portainer_kubernetes_endpoints" "all" {
  endpoint_id = portainer_environment.prod.id
}

output "services_with_no_backends" {
  value = [
    for e in data.portainer_kubernetes_endpoints.all.endpoints : "${e.namespace}/${e.name}"
    if length(e.addresses) == 0
  ]
}
```

## Arguments Reference

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `endpoint_id` | number | ✅ yes | Identifier of the Kubernetes environment to query. |

## Attributes Reference

| Name | Type | Description |
|------|------|-------------|
| `endpoints` | list(object) | The Kubernetes Endpoints objects in the cluster - the addresses behind each service. |

### `endpoints`

| Name | Type | Description |
|------|------|-------------|
| `addresses` | list(string) | Addresses behind the service. An empty list means the service has no ready backends. |
| `name` | string | Name of the Endpoints object, which matches the service it backs. |
| `namespace` | string | Namespace of the Endpoints object. |
| `ports` | list(object) | Ports the endpoints expose. |
| `uid` | string | Kubernetes UID of the Endpoints object. |

### `endpoints.ports`

| Name | Type | Description |
|------|------|-------------|
| `name` | string | Name of the port. |
| `port` | number | Port number. |
| `protocol` | string | Protocol of the port, for example `TCP`. |

An Endpoints object with an empty `addresses` list is a service with no ready backends, which is usually the thing worth finding here.
