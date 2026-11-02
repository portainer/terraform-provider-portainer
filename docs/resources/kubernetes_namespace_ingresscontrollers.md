# 🌐 **Resource Documentation: `portainer_kubernetes_namespace_ingresscontrollers`**

# portainer_kubernetes_namespace_ingresscontrollers
The `portainer_kubernetes_namespace_ingresscontrollers` resource allows you to manage (block/unblock) ingress controllers per namespace in a Kubernetes environment within Portainer.

## Example Usage
```hcl
resource "portainer_kubernetes_namespace_ingresscontrollers" "test" {
  environment_id = 1
  namespace      = "default"

  controllers {
    name         = "nginx"
    class_name   = "nginx"
    type         = "nginx"
    availability = true
    used         = true
    new          = false
  }
}
```

## Lifecycle & Behavior
- This resource updates ingress controller configurations via the Portainer API.
- You can enable, disable or modify ingress controllers declaratively via Terraform; changes apply in place.
- Per-namespace availability only takes effect when the environment has `ingress_availability_per_namespace` enabled (see `portainer_kubernetes_environment_config`); otherwise Portainer reports every controller as available.
- A controller blocked for the whole environment is not listed for the namespace and shows as drift.
- Portainer only takes `class_name` and `availability`; `name`, `type`, `used` and `new` are informational and a difference in them is not planned once the controller is in state. Every `class_name` must exist in the cluster, otherwise the apply fails.
- Destroying the resource makes the listed controllers available in the namespace again (Portainer's default).

### Arguments Reference
| Name             | Type   | Required | Description                                           |
|------------------|--------|----------|-------------------------------------------------------|
| `environment_id` | number | ✅ yes   | ID of the Portainer environment (endpoint).          |
| `namespace`      | string | ✅ yes   | Namespace within the Kubernetes cluster.             |
| `controllers`    | block  | ✅ yes   | A block defining a controller configuration.         |

#### `controllers` block
| Name          | Type   | Required | Description                                             |
|---------------|--------|----------|---------------------------------------------------------|
| `name`        | string | ✅ yes   | Name of the ingress controller. Informational.          |
| `class_name`  | string | ✅ yes   | Class name of the ingress controller. Must exist in the cluster. |
| `type`        | string | ✅ yes   | Controller type (e.g., `nginx`). Informational.         |
| `availability`| bool   | ✅ yes   | Whether the controller is available.                    |
| `used`        | bool   | ✅ yes   | Whether the controller is used in this namespace. Informational. |
| `new`         | bool   | ✅ yes   | Whether the controller is newly added. Informational.   |

---

### Attributes Reference

| Name | Description                               |
|------|-------------------------------------------|
| `id` | Unique identifier for the kubernetes namespace ingresscontrollers    |

## Import

Namespace ingress controller settings can be imported using `environment_id:namespace`:

```shell
terraform import portainer_kubernetes_namespace_ingresscontrollers.example 4:default
```
