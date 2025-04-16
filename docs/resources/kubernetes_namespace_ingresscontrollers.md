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
- You can enable, disable or modify ingress controllers declaratively via Terraform.

### Arguments Reference
| Name             | Type   | Required | Description                                           |
|------------------|--------|----------|-------------------------------------------------------|
| `environment_id` | number | ✅ yes   | ID of the Portainer environment (endpoint).          |
| `namespace`      | string | ✅ yes   | Namespace within the Kubernetes cluster.             |
| `controllers`    | block  | ✅ yes   | A block defining a controller configuration.         |

#### `controllers` block
| Name          | Type   | Required | Description                                             |
|---------------|--------|----------|---------------------------------------------------------|
| `name`        | string | ✅ yes   | Name of the ingress controller.                         |
| `class_name`  | string | ✅ yes   | Class name of the ingress controller.                   |
| `type`        | string | ✅ yes   | Controller type (e.g., `nginx`).                        |
| `availability`| bool   | ✅ yes   | Whether the controller is available.                    |
| `used`        | bool   | ✅ yes   | Whether the controller is used in this namespace.       |
| `new`         | bool   | ✅ yes   | Whether the controller is newly added.                  |

---

### Attributes Reference

| Name | Description                               |
|------|-------------------------------------------|
| `id` | Unique identifier for the kubernetes namespace ingresscontrollers    |
