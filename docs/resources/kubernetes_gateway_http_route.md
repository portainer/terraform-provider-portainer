# Resource Documentation: `portainer_kubernetes_gateway_http_route`

# portainer_kubernetes_gateway_http_route

> **Requires Portainer Business Edition 3.0 or later.** On Community Edition or an older server the create fails with an error that says so.

Manages a Kubernetes Gateway API `HTTPRoute` through Portainer's own Kubernetes API: which HTTP requests a Gateway (`portainer_kubernetes_gateway`) accepts are forwarded to which Services, with optional header changes and redirects.

> Alternatively, Gateway API objects (including kinds this resource does not model, such as `GRPCRoute` or `ReferenceGrant`, and HTTPRoute features Portainer's model leaves out, such as URL rewrites or request mirroring) can be managed as raw manifests instead: a `portainer_stack` with `deployment_type = "kubernetes"` deploys any manifest, and `portainer_kubernetes_helm` deploys a chart that contains them.

## Example Usage

```hcl
resource "portainer_kubernetes_gateway_http_route" "app" {
  environment_id = 4
  namespace      = "web"
  name           = "app"
  hostnames      = ["app.example.com"]

  parent_ref {
    name         = portainer_kubernetes_gateway.edge.name
    section_name = "http"
  }

  rule {
    match {
      path_type  = "PathPrefix"
      path_value = "/api"
    }

    filter {
      type = "RequestHeaderModifier"
      request_header_modifier {
        set {
          name  = "x-forwarded-by"
          value = "gateway"
        }
        remove = ["x-debug"]
      }
    }

    backend_ref {
      name = "api"
      port = 8080
    }
  }

  # Plain HTTP to HTTPS redirect for everything else.
  rule {
    filter {
      type = "RequestRedirect"
      request_redirect {
        scheme      = "https"
        status_code = 301
      }
    }
  }
}
```

## Lifecycle & Behavior

* Changing `environment_id`, `namespace` or `name` replaces the route; every other argument is updated in place, sending the `resource_version` read at refresh time so a concurrent change fails the apply instead of being overwritten.
* Portainer has no read of a single route, so a refresh lists the namespace's routes and picks this one by name. A route no longer listed is removed from state and recreated on the next apply.
* Every argument is refreshed from the cluster. Fields Kubernetes fills in with a default when unset (`parent_ref.group`, `match` and its `path_type`/`path_value`, `header`/`query_param` `type`, `backend_ref.kind`/`group`) take the server's value and do not show as a diff.
* Deleting the resource deletes the route (Portainer's bulk delete, with this route as its only item).

## Arguments Reference

| Name             | Type         | Required    | Description |
|------------------|--------------|-------------|-------------|
| `environment_id` | int          | ✅ yes       | ID of the Portainer Kubernetes environment. Changing it forces a new resource. |
| `namespace`      | string       | ✅ yes       | Namespace of the route. Changing it forces a new resource. |
| `name`           | string       | ✅ yes       | Name of the route. Changing it forces a new resource. |
| `hostnames`      | list(string) | 🚫 optional | Hostnames matched against the Host header; unset matches every hostname the listener accepts. |
| `parent_ref`     | block(list)  | ✅ yes       | Gateways (or listeners of them) the route attaches to, at least one. |
| `rule`           | block(list)  | 🚫 optional | Routing rules. |

### `parent_ref` block

| Name           | Type   | Required    | Description |
|----------------|--------|-------------|-------------|
| `name`         | string | ✅ yes       | Name of the Gateway. |
| `namespace`    | string | 🚫 optional | Namespace of the Gateway; defaults to the route's. |
| `section_name` | string | 🚫 optional | Listener to attach to. |
| `port`         | int    | 🚫 optional | Listener port to attach to. |
| `group`        | string | 🚫 optional | API group of the parent; Kubernetes defaults it to `gateway.networking.k8s.io`. |

### `rule` block

| Name          | Type        | Required    | Description |
|---------------|-------------|-------------|-------------|
| `name`        | string      | 🚫 optional | Name of the rule. |
| `match`       | block(list) | 🚫 optional | Conditions, any of which matches: `path_type` (`Exact`, `PathPrefix`, `RegularExpression`), `path_value`, `method`, `header { name, type, value }`, `query_param { name, type, value }`. Kubernetes defaults an empty list to `PathPrefix` `/`. |
| `filter`      | block(list) | 🚫 optional | Filters for every matching request (see below). |
| `backend_ref` | block(list) | 🚫 optional | Backends: `name` (required), `namespace`, `port`, `kind` (default `Service`), `group`, `weight` (default `1`; `0` sends no traffic) and backend-only `filter` blocks. |

### `filter` block

| Name                      | Type        | Required    | Description |
|---------------------------|-------------|-------------|-------------|
| `type`                    | string      | ✅ yes       | `RequestHeaderModifier` or `RequestRedirect`. |
| `request_header_modifier` | block       | 🚫 optional | `add { name, value }`, `set { name, value }` and `remove` (list of header names). |
| `request_redirect`        | block       | 🚫 optional | `hostname`, `port`, `scheme`, `status_code` (`301` or `302`). |

## Attributes Reference

| Name               | Type   | Description |
|--------------------|--------|-------------|
| `id`               | string | `<environment_id>:<namespace>:<name>`. |
| `resource_version` | string | Kubernetes resource version, sent back on update. |

## Import

HTTPRoutes can be imported with `<environment_id>:<namespace>:<name>`; every argument is read back from the cluster:

```shell
terraform import portainer_kubernetes_gateway_http_route.app 4:web:app
```
