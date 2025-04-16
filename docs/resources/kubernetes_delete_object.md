# 🗑️ **Resource Documentation: `portainer_kubernetes_delete_object`**

# portainer_kubernetes_delete_object
The `portainer_kubernetes_delete_object` resource allows you to delete Kubernetes objects (e.g. services, ingresses, jobs) within a specific environment (endpoint) and namespace in Portainer.
> ⚠️ This is a destructive action. Use carefully – resources deleted through this resource cannot be recovered via Terraform.

## Example Usage
```hcl
resource "portainer_kubernetes_delete_object" "remove_services" {
  environment_id = 4
  resource_type  = "services"
  namespace      = "default"
  names          = ["service-name-1"]
}
```

## Lifecycle & Behavior
- This resource performs a POST request to the Portainer API to delete the selected objects.

### Arguments Reference
| Name             | Type         | Required | Description                                                                                      |
|------------------|--------------|----------|--------------------------------------------------------------------------------------------------|
| `environment_id` | number       | ✅ yes   | ID of the Portainer Kubernetes environment (endpoint).                                           |
| `resource_type`  | string       | ✅ yes   | Type of object to delete. One of: `services`, `ingresses`, `jobs`, `cron_jobs`, `roles`, `role_bindings`, `service_accounts`, `cluster_role_bindings`, `cluster_roles`. |
| `namespace`      | string       | ✅ yes   | Kubernetes namespace where the objects reside.                                                   |
| `names`          | list(string) | ✅ yes   | List of object names to delete.                                                                  |

---

### Attributes Reference

| Name | Description                               |
|------|-------------------------------------------|
| `id` | Unique identifier for the delete object   |
