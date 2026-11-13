# Resource Documentation: `portainer_kubernetes_gateway`

# portainer_kubernetes_gateway

> **Requires Portainer Business Edition 3.0 or later.** On Community Edition or an older server the create fails with an error that says so.

Manages a Kubernetes Gateway API `Gateway` (`gateway.networking.k8s.io`) through Portainer's own Kubernetes API, the same way `portainer_kubernetes_ingresses` manages Ingresses. Routes attach to it with `portainer_kubernetes_gateway_http_route`.

The cluster needs the Gateway API CRDs and a Gateway API implementation (Envoy Gateway, Istio, Cilium, Traefik, ...) providing a GatewayClass; list them with the `portainer_kubernetes_gateway_classes` data source.

> Alternatively, Gateway API objects (including kinds this resource does not model, such as `TLSRoute`, `GRPCRoute` or `ReferenceGrant`) can be managed as raw manifests instead: a `portainer_stack` with `deployment_type = "kubernetes"` deploys any manifest, and `portainer_kubernetes_helm` deploys a chart that contains them.

## Example Usage

```hcl
data "portainer_kubernetes_gateway_classes" "all" {
  environment_id = 4
}

resource "portainer_kubernetes_gateway" "edge" {
  environment_id = 4
  namespace      = "web"
  name           = "edge"
  class_name     = data.portainer_kubernetes_gateway_classes.all.gateway_classes[0].name

  listener {
    name     = "http"
    port     = 80
    protocol = "HTTP"
    hostname = "*.example.com"
  }

  labels = {
    team = "web"
  }
}
```

## Lifecycle & Behavior

* Changing `environment_id`, `namespace` or `name` replaces the Gateway; every other argument is updated in place.
* An update sends the `resource_version` read at refresh time, so a change made to the Gateway in the meantime fails the apply with a conflict instead of being overwritten; the next plan shows it.
* Portainer has no read of a single Gateway, so a refresh lists the namespace's Gateways and picks this one by name. A Gateway that is no longer listed is removed from state and recreated on the next apply.
* Every argument is refreshed from the cluster, so changes made outside Terraform show as drift.
* Deleting the resource deletes the Gateway (Portainer's bulk delete, with this Gateway as its only item).

## Arguments Reference

| Name             | Type        | Required    | Description |
|------------------|-------------|-------------|-------------|
| `environment_id` | int         | ✅ yes       | ID of the Portainer Kubernetes environment. Changing it forces a new resource. |
| `namespace`      | string      | ✅ yes       | Namespace of the Gateway. Changing it forces a new resource. |
| `name`           | string      | ✅ yes       | Name of the Gateway. Changing it forces a new resource. |
| `class_name`     | string      | ✅ yes       | GatewayClass implementing the Gateway (`spec.gatewayClassName`). |
| `listener`       | block(list) | ✅ yes       | Listeners of the Gateway, at least one. |
| `address`        | block(list) | 🚫 optional | Addresses requested for the Gateway (`spec.addresses`). |
| `annotations`    | map(string) | 🚫 optional | Annotations applied to the Gateway. |
| `labels`         | map(string) | 🚫 optional | Labels applied to the Gateway. |

### `listener` block

| Name       | Type   | Required    | Description |
|------------|--------|-------------|-------------|
| `name`     | string | ✅ yes       | Name of the listener, unique within the Gateway; an HTTPRoute attaches to it with `parent_ref.section_name`. |
| `port`     | int    | ✅ yes       | Port the listener accepts traffic on. |
| `protocol` | string | ✅ yes       | Protocol, for example `HTTP` or `HTTPS`. |
| `hostname` | string | 🚫 optional | Hostname the listener matches, for example `*.example.com`. |

### `address` block

| Name    | Type   | Required    | Description |
|---------|--------|-------------|-------------|
| `type`  | string | 🚫 optional | Type of the address, for example `IPAddress` or `Hostname`. |
| `value` | string | ✅ yes       | The address. |

## Attributes Reference

| Name               | Type   | Description |
|--------------------|--------|-------------|
| `id`               | string | `<environment_id>:<namespace>:<name>`. |
| `uid`              | string | Kubernetes UID of the Gateway. |
| `resource_version` | string | Kubernetes resource version, sent back on update. |
| `creation_date`    | string | Time the Gateway was created. |

## Import

Gateways can be imported with `<environment_id>:<namespace>:<name>`; every argument is read back from the cluster:

```shell
terraform import portainer_kubernetes_gateway.edge 4:web:edge
```
