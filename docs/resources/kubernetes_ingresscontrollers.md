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

### Arguments Reference
| Name             | Type   | Required | Description                                                     |
|------------------|--------|----------|-----------------------------------------------------------------|
| `environment_id` | number | ✅ yes   | The environment (endpoint) ID for the Kubernetes cluster.       |
| `controllers`    | block  | ✅ yes   | One or more ingress controller configurations.                  |

#### `controllers` block
| Field         | Type    | Required | Description                                                                 |
|---------------|---------|----------|-----------------------------------------------------------------------------|
| `name`        | string  | ✅ yes   | Name of the ingress controller.                                             |
| `class_name`  | string  | ✅ yes   | Class name used by the ingress controller (e.g. `"nginx"`).                |
| `type`        | string  | ✅ yes   | Type of the ingress controller (e.g. `"nginx"`).                           |
| `availability`| bool    | ✅ yes   | Whether the controller is available.                                        |
| `used`        | bool    | ✅ yes   | Whether the controller is actively used.                                    |
| `new`         | bool    | ✅ yes   | Marks the controller as newly created.                                     |

---

### Attributes Reference

| Name | Description                               |
|------|-------------------------------------------|
| `id` | Unique identifier for the kubernetes ingresscontrollers    |
