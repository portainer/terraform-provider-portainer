# 🧩 **Resource Documentation: `portainer_kubernetes_namespace`**

# portainer_kubernetes_namespace
The `portainer_kubernetes_namespace` resource allows you to manage Kubernetes namespaces within a specific environment (endpoint) in Portainer.

## Example Usage
```hcl
resource "portainer_kubernetes_namespace" "test" {
  environment_id = 4
  name           = "testest"
  owner          = "terraform-test"

  annotations = {
    "owner" = "terraform"
    "env"   = "test"
  }

  resource_quota = {
    cpu    = "800m"
    memory = "129Mi"
  }
}
```

## Lifecycle & Behavior
- Terraform updates the namespace if `owner`, `annotations`, or `resource_quota` change.
- Changing the `name` will trigger a destroy and recreate operation due to API limitations (Portainer does not support renaming namespaces).
- You can use `terraform destroy` to delete the namespace completely.

### Arguments Reference
| Name             | Type   | Required                     | Description                                                              |
|------------------|--------|------------------------------|--------------------------------------------------------------------------|
| `environment_id` | number | ✅ yes                       | ID of the Portainer environment (Kubernetes endpoint).                   |
| `name`           | string | ✅ yes                       | Name of the Kubernetes namespace.                                        |
| `owner`          | string | 🚫 optional (default: `""`) | Optional owner string shown in the namespace info.                       |
| `annotations`    | map    | 🚫 optional                  | Map of annotations to apply to the namespace.                            |
| `resource_quota` | object | 🚫 optional                  | CPU and memory quota (requires keys: `cpu` and `memory`).                |

---

### Attributes Reference

| Name  | Description                                  |
|-------|----------------------------------------------|
| `id`  | Composite ID in format `environmentID:name`  |
