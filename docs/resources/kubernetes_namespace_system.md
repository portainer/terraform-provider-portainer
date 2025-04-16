# 🛡️ **Resource Documentation: `portainer_kubernetes_namespace_system`**

# portainer_kubernetes_namespace_system
The `portainer_kubernetes_namespace_system` resource allows you to toggle the "system" flag for a Kubernetes namespace in Portainer. This flag marks the namespace as a system-managed one, which can be useful for visibility filtering and administrative controls.

## Example Usage
```hcl
resource "portainer_kubernetes_namespace_system" "system_flag" {
  environment_id = 4
  namespace      = "kube-system"
  system         = true
}
```

## Lifecycle & Behavior
- This resource uses a `PUT` request to set the system state of a namespace.
- Portainer does not provide a `GET` method to read the current state, so this resource is write-only.
- Terraform will not detect drift, but you can re-apply to ensure idempotency.

### Arguments Reference
| Name             | Type   | Required | Description                                                          |
|------------------|--------|----------|----------------------------------------------------------------------|
| `environment_id` | number | ✅ yes   | ID of the Portainer environment (Kubernetes endpoint).              |
| `namespace`      | string | ✅ yes   | The name of the Kubernetes namespace to update.                     |
| `system`         | bool   | ✅ yes   | Whether the namespace should be marked as a system namespace.       |

---

### Attributes Reference
| Name  | Description                                              |
|-------|----------------------------------------------------------|
| `id`  | Composite ID in format `environment_id:namespace`.       |
