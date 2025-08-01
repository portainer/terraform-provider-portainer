# 🔐 **Resource Documentation: `portainer_kubernetes_namespace_access`**

## `portainer_kubernetes_namespace_access`

The `portainer_kubernetes_namespace_access` resource allows you to configure user and team access permissions to specific Kubernetes namespaces managed by Portainer.

This is useful when you want to manage granular RBAC controls for Kubernetes namespaces via Terraform.

---

## 🚀 Example Usage

### Grant access to a namespace for specific users and teams

```hcl
resource "portainer_kubernetes_namespace_access" "example" {
  endpoint_id = 1
  namespace   = 2

  users_to_add     = [3, 5]
  users_to_remove  = []

  teams_to_add     = [7]
  teams_to_remove  = []
}
```

---

## ⚙️ Lifecycle & Behavior

Access is **applied on every `terraform apply`**, but only triggers the update request if:
- The `type` of the environment (endpoint) is Kubernetes (`type = 5`)
- At least one of the `*_to_add` or `*_to_remove` fields is set

---

## 📥 Arguments Reference

| Name              | Type         | Required | Description                                                                 |
|-------------------|--------------|----------|-----------------------------------------------------------------------------|
| `endpoint_id`     | number       | ✅ yes   | ID of the Portainer environment (endpoint)                                  |
| `namespace`       | number       | ✅ yes   | ID of the Kubernetes namespace (called RPN) in Portainer                    |
| `users_to_add`    | list(number) | 🚫 no    | List of user IDs to be granted access to the namespace                      |
| `users_to_remove` | list(number) | 🚫 no    | List of user IDs to be revoked from the namespace                           |
| `teams_to_add`    | list(number) | 🚫 no    | List of team IDs to be granted access to the namespace                      |
| `teams_to_remove` | list(number) | 🚫 no    | List of team IDs to be revoked from the namespace                           |

---

## 📤 Attributes Reference

| Name | Description                                                    |
|------|----------------------------------------------------------------|
| `id` | Unique identifier in format `namespace-access-{endpoint_id}-{namespace}` |
