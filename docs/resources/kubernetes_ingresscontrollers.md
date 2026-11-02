# 🌐 **Resource Documentation: `portainer_kubernetes_ingresscontrollers`**

# portainer_kubernetes_ingresscontrollers
The `portainer_kubernetes_ingresscontrollers` resource allows you to manage and update ingress controllers for a Kubernetes environment in Portainer.

This includes setting the availability, type, and usage of specific ingress controllers.

## Example Usage
```hcl
resource "portainer_kubernetes_ingresscontrollers" "test" {
  environment_id = 1

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
- Portainer only takes `class_name` and `availability`; it fills `name`, `type`, `used` and `new` in itself, so they are informational and a difference in them is not planned once the controller is in state.
- Only the listed classes are managed; other controllers in the cluster are left alone. Every `class_name` must exist in the cluster, otherwise the apply fails.
- Destroying the resource makes the listed controllers available again (Portainer's default).

### Arguments Reference
| Name             | Type   | Required | Description                                                     |
|------------------|--------|----------|-----------------------------------------------------------------|
| `environment_id` | number | ✅ yes   | The environment (endpoint) ID for the Kubernetes cluster.       |
| `controllers`    | block  | ✅ yes   | One or more ingress controller configurations.                  |

#### `controllers` block
| Field         | Type    | Required | Description                                                                 |
|---------------|---------|----------|-----------------------------------------------------------------------------|
| `name`        | string  | ✅ yes   | Name of the ingress controller. Informational.                              |
| `class_name`  | string  | ✅ yes   | Class name used by the ingress controller (e.g. `"nginx"`). Must exist in the cluster. |
| `type`        | string  | ✅ yes   | Type of the ingress controller (e.g. `"nginx"`). Informational.            |
| `availability`| bool    | ✅ yes   | Whether the controller is available.                                        |
| `used`        | bool    | ✅ yes   | Whether the controller is actively used. Informational.                     |
| `new`         | bool    | ✅ yes   | Marks the controller as newly created. Informational.                      |

---

### Attributes Reference

| Name | Description                               |
|------|-------------------------------------------|
| `id` | Unique identifier for the kubernetes ingresscontrollers    |

## Import

Ingress controller settings can be imported using the numeric environment ID:

```shell
terraform import portainer_kubernetes_ingresscontrollers.example 5
```
